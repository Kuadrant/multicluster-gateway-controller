package dnshealthcheckprobe

import (
	"context"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/health"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatusUpdateProbeNotifier struct {
	apiClient   client.Client
	probeObjKey client.ObjectKey
}

var _ health.ProbeNotifier = StatusUpdateProbeNotifier{}

func NewStatusUpdateProbeNotifier(apiClient client.Client, forObj *v1alpha1.DNSHealthCheckProbe) StatusUpdateProbeNotifier {
	return StatusUpdateProbeNotifier{
		apiClient:   apiClient,
		probeObjKey: client.ObjectKeyFromObject(forObj),
	}
}

func (n StatusUpdateProbeNotifier) Notify(ctx context.Context, result health.ProbeResult) error {
	probeObj := &v1alpha1.DNSHealthCheckProbe{}
	if err := n.apiClient.Get(ctx, n.probeObjKey, probeObj); err != nil {
		return err
	}

	// Increase the number of consecutive failures if it failed previously
	if !result.Healthy {
		if probeObj.Status.Healthy {
			probeObj.Status.ConsecutiveFailures = 1
		} else {
			probeObj.Status.ConsecutiveFailures++
		}
	} else {
		probeObj.Status.ConsecutiveFailures = 0
	}

	probeObj.Status.LastCheckedAt = metav1.NewTime(result.CheckedAt)
	probeObj.Status.Healthy = result.Healthy
	probeObj.Status.Reason = result.Reason

	return n.apiClient.Status().Update(ctx, probeObj)
}
