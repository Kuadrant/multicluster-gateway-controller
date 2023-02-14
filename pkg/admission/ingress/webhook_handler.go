package dnsrecord

import (
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/ingress"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"

	trafficadmission "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/admission/traffic"
)

type Handler struct {
	*trafficadmission.TrafficWebhookHandler[*networkingv1.Ingress]
}

func CreateHandler(hostService ingress.HostService, certService ingress.CertificateService) (admission.Handler, error) {
	trafficHandler, err := trafficadmission.NewTrafficWebhookHandler(
		networkingv1.AddToScheme,
		func() *networkingv1.Ingress { return &networkingv1.Ingress{} },
		traffic.NewIngress,
		hostService,
		certService,
	)
	if err != nil {
		return nil, err
	}

	return &Handler{trafficHandler}, nil
}
