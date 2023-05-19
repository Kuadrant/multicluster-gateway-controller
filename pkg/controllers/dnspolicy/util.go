package dnspolicy

import (
	"context"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func getGateway(ctx context.Context, apiClient client.Client, policy *v1alpha1.DNSPolicy) (*gatewayv1beta1.Gateway, error) {
	key := client.ObjectKey{
		Name:      string(policy.Spec.TargetRef.Name),
		Namespace: policy.Namespace,
	}

	gateway := &gatewayv1beta1.Gateway{}

	return gateway, apiClient.Get(ctx, key, gateway)
}

func getDNSRecords(ctx context.Context, apiClient client.Client, hostService gateway.HostService, policy *v1alpha1.DNSPolicy) ([]*v1alpha1.DNSRecord, error) {
	gateway, err := getGateway(ctx, apiClient, policy)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return make([]*v1alpha1.DNSRecord, 0), nil
		}

		return nil, err
	}

	trafficAccessor := traffic.NewGateway(gateway)
	return hostService.GetDNSRecordsFor(ctx, trafficAccessor)
}
