package dnshealthcheckprobe

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/health"
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

func (n StatusUpdateProbeNotifier) Notify(ctx context.Context, result health.ProbeResult) (health.NotificationResult, error) {
	probeObj := &v1alpha1.DNSHealthCheckProbe{}
	if err := n.apiClient.Get(ctx, n.probeObjKey, probeObj); err != nil {
		return health.NotificationResult{}, err
	}

	// Increase the number of consecutive failures if it failed previously
	if !result.Healthy {
		probeHealthy := true
		if probeObj.Status.Healthy != nil {
			probeHealthy = *probeObj.Status.Healthy
		}
		if probeHealthy {
			probeObj.Status.ConsecutiveFailures = 1
		} else {
			probeObj.Status.ConsecutiveFailures++
		}
	} else {
		probeObj.Status.ConsecutiveFailures = 0
	}

	probeObj.Status.LastCheckedAt = metav1.NewTime(result.CheckedAt)
	if probeObj.Status.Healthy == nil {
		probeObj.Status.Healthy = aws.Bool(true)
	}
	probeObj.Status.Healthy = &result.Healthy
	probeObj.Status.Reason = result.Reason
	probeObj.Status.Status = result.Status

	if err := n.apiClient.Status().Update(ctx, probeObj); err != nil {
		if errors.IsConflict(err) {
			return health.NotificationResult{Requeue: true}, nil
		}

		return health.NotificationResult{}, err
	}

	return health.NotificationResult{}, nil
}
