package dnsrecord

import (
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trafficadmission "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/admission/traffic"
	controllertraffic "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/traffic"
)

type Handler struct {
	*trafficadmission.TrafficWebhookHandler[*networkingv1.Ingress]
}

func CreateHandler(hostService controllertraffic.HostService, certService controllertraffic.CertificateService) (admission.Handler, error) {
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
