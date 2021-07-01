package reviewer

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/traefik/hub-agent/pkg/acp"
	"github.com/traefik/hub-agent/pkg/acp/admission/ingclass"
	"github.com/traefik/hub-agent/pkg/acp/admission/quota"
	admv1 "k8s.io/api/admission/v1"
)

const annotationTraefikMiddlewares = "traefik.ingress.kubernetes.io/router.middlewares"

// TraefikIngress is a reviewer that can handle Traefik ingress resources.
// Note that this reviewer requires Traefik middleware CRD to be defined in the cluster.
// It also requires Traefik to have the Kubernetes CRD provider enabled.
type TraefikIngress struct {
	ingressClasses     IngressClasses
	fwdAuthMiddlewares FwdAuthMiddlewares
	quotas             QuotaTransaction
}

// NewTraefikIngress returns a Traefik ingress reviewer.
func NewTraefikIngress(ingClasses IngressClasses, fwdAuthMiddlewares FwdAuthMiddlewares, quotas QuotaTransaction) *TraefikIngress {
	return &TraefikIngress{
		ingressClasses:     ingClasses,
		fwdAuthMiddlewares: fwdAuthMiddlewares,
		quotas:             quotas,
	}
}

// CanReview returns whether this reviewer can handle the given admission review request.
func (r TraefikIngress) CanReview(ar admv1.AdmissionReview) (bool, error) {
	resource := ar.Request.Kind

	// Check resource type. Only continue if it's a legacy Ingress (<1.18) or an Ingress resource.
	if !isNetV1Ingress(resource) && !isNetV1Beta1Ingress(resource) && !isExtV1Beta1Ingress(resource) {
		return false, nil
	}

	obj := ar.Request.Object.Raw
	if ar.Request.Operation == admv1.Delete {
		obj = ar.Request.OldObject.Raw
	}
	ingClassName, ingClassAnno, err := parseIngressClass(obj)
	if err != nil {
		return false, fmt.Errorf("parse raw ingress class: %w", err)
	}

	defaultCtrlr, err := r.ingressClasses.GetDefaultController()
	if err != nil {
		return false, fmt.Errorf("get default ingress class controller: %w", err)
	}

	var ctrlr string
	switch {
	case ingClassName != "":
		ctrlr, err = r.ingressClasses.GetController(ingClassName)
		if err != nil {
			return false, fmt.Errorf("get ingress class controller from ingress class name: %w", err)
		}
		return isTraefik(ctrlr), nil
	case ingClassAnno != "":
		if ingClassAnno == defaultAnnotationTraefik {
			return true, nil
		}

		// Don't return an error if it's the default value of another reviewer,
		// just say we can't review it.
		if isDefaultIngressClassValue(ingClassAnno) {
			return false, nil
		}

		ctrlr, err = r.ingressClasses.GetController(ingClassAnno)
		if err != nil {
			return false, fmt.Errorf("get ingress class controller from annotation: %w", err)
		}
		return isTraefik(ctrlr), nil
	default:
		return isTraefik(defaultCtrlr), nil
	}
}

// Review reviews the given admission review request and optionally returns the required patch.
func (r TraefikIngress) Review(ctx context.Context, ar admv1.AdmissionReview) (map[string]interface{}, error) {
	l := log.Ctx(ctx).With().Str("reviewer", "TraefikIngress").Logger()
	ctx = l.WithContext(ctx)

	log.Ctx(ctx).Info().Msg("Reviewing Ingress resource")

	if ar.Request.Operation == admv1.Delete {
		log.Ctx(ctx).Info().Msg("Deleting Ingress resource")
		if err := releaseQuotas(r.quotas, ar.Request.Name, ar.Request.Namespace); err != nil {
			return nil, err
		}
		return nil, nil
	}

	ing, oldIng, err := parseRawIngresses(ar.Request.Object.Raw, ar.Request.OldObject.Raw)
	if err != nil {
		return nil, fmt.Errorf("parse raw objects: %w", err)
	}

	prevPolName := oldIng.Metadata.Annotations[AnnotationHubAuth]
	polName := ing.Metadata.Annotations[AnnotationHubAuth]

	if prevPolName != "" && polName == "" {
		if err = releaseQuotas(r.quotas, ar.Request.Name, ar.Request.Namespace); err != nil {
			return nil, err
		}
	}

	if prevPolName == "" && polName == "" {
		log.Ctx(ctx).Debug().Msg("No ACP defined")
		return nil, nil
	}

	routerMiddlewares := ing.Metadata.Annotations[annotationTraefikMiddlewares]

	if prevPolName != "" {
		routerMiddlewares, err = r.clearPreviousFwdAuthMiddleware(ctx, prevPolName, ing.Metadata.Namespace, routerMiddlewares)
		if err != nil {
			return nil, err
		}
	}

	if polName != "" {
		var tx *quota.Tx
		tx, err = r.quotas.Tx(resourceID(ar.Request.Name, ar.Request.Namespace), countRoutes(ing.Spec))
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				tx.Rollback()
			} else {
				tx.Commit()
			}
		}()

		var middlewareName string
		middlewareName, err = r.fwdAuthMiddlewares.Setup(ctx, polName, ing.Metadata.Namespace)
		if err != nil {
			return nil, err
		}

		routerMiddlewares = appendMiddleware(
			routerMiddlewares,
			fmt.Sprintf("%s-%s@kubernetescrd", ing.Metadata.Namespace, middlewareName),
		)
	}

	if ing.Metadata.Annotations[annotationTraefikMiddlewares] == routerMiddlewares {
		log.Ctx(ctx).Debug().Str("acp_name", polName).Msg("No patch required")
		return nil, nil
	}

	if routerMiddlewares != "" {
		ing.Metadata.Annotations[annotationTraefikMiddlewares] = routerMiddlewares
	} else {
		delete(ing.Metadata.Annotations, annotationTraefikMiddlewares)
	}

	log.Ctx(ctx).Info().Str("acp_name", polName).Msg("Patching resource")

	return map[string]interface{}{
		"op":    "replace",
		"path":  "/metadata/annotations",
		"value": ing.Metadata.Annotations,
	}, nil
}

func (r TraefikIngress) clearPreviousFwdAuthMiddleware(ctx context.Context, polName, namespace, routerMiddlewares string) (string, error) {
	log.Ctx(ctx).Debug().Str("prev_acp_name", polName).Msg("Clearing previous ACP settings")

	canonicalOldPolName, err := acp.CanonicalName(polName, namespace)
	if err != nil {
		return "", err
	}

	middlewareName := middlewareName(canonicalOldPolName)
	oldCanonicalMiddlewareName := fmt.Sprintf("%s-%s@kubernetescrd", namespace, middlewareName)

	return removeMiddleware(routerMiddlewares, oldCanonicalMiddlewareName), nil
}

// appendMiddleware appends newMiddleware to the comma-separated list of middlewareList.
func appendMiddleware(middlewareList, newMiddleware string) string {
	if middlewareList == "" {
		return newMiddleware
	}

	return middlewareList + "," + newMiddleware
}

// removeMiddleware removes the middleware named toRemove from the given middlewareList, if found.
func removeMiddleware(middlewareList, toRemove string) string {
	var res []string

	for _, m := range strings.Split(middlewareList, ",") {
		if m != toRemove {
			res = append(res, m)
		}
	}

	return strings.Join(res, ",")
}

// middlewareName returns the ForwardAuth middleware desc for the given ACP.
func middlewareName(polName string) string {
	return fmt.Sprintf("zz-%s", strings.ReplaceAll(polName, "@", "-"))
}

func isTraefik(ctrlr string) bool {
	return ctrlr == ingclass.ControllerTypeTraefik
}
