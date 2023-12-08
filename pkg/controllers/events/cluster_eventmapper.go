package events

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
)

// ClusterEventMapper is an EventHandler that maps Cluster object events to gateway events.
//
// Cluster object can be anything that represents a cluster and has mgc attribute labels applied to (e.g. OCM ManagedCluster)
type ClusterEventMapper struct {
	Logger logr.Logger
	Client client.Client
}

func NewClusterEventMapper(logger logr.Logger, client client.Client) *ClusterEventMapper {
	log := logger.WithName("ClusterEventMapper")
	return &ClusterEventMapper{
		Logger: log,
		Client: client,
	}
}

func (m *ClusterEventMapper) MapToGateway(ctx context.Context, obj client.Object) []reconcile.Request {
	return m.mapToGatewayRequest(ctx, obj)
}

func (m *ClusterEventMapper) mapToGatewayRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := m.Logger.V(1).WithValues("object", client.ObjectKeyFromObject(obj))

	if obj.GetDeletionTimestamp() != nil {
		// Ignore ManagedCluster delete events.
		// Create/Update events are OK as ManagedCluster custom attributes may change, affecting DNSPolicies.
		// However, deleting a ManagedCluster shouldn't affect the DNSPolicy directly until the related
		// Gateway is deleted from that ManagedCluster (and reconciled then via watching Gateways, not ManagedClusters)
		return []reconcile.Request{}
	}

	clusterName := obj.GetName()

	allGwList := &gatewayapiv1.GatewayList{}
	err := m.Client.List(ctx, allGwList)
	if err != nil {
		logger.Info("mapToPolicyRequest:", "error", "failed to get gateways")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)
	for _, gw := range allGwList.Items {
		val := metadata.GetAnnotation(&gw, "kuadrant.io/gateway-clusters")
		if val == "" {
			continue
		}
		var clusters []string
		if err = json.Unmarshal([]byte(val), &clusters); err == nil {
			if slice.ContainsString(clusters, clusterName) {
				requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&gw)})
			}
		}
	}

	return requests
}
