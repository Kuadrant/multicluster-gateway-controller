package dnspolicy

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayapiv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
)

// ClusterEventMapper is an EventHandler that maps Cluster object events to policy events.
//
// Cluster object can be anything that represents a cluster and has mgc attribute labels applied to (e.g. OCM ManagedCluster)
type ClusterEventMapper struct {
	Logger             logr.Logger
	GatewayEventMapper *GatewayEventMapper
	client             client.Client
}

func (m *ClusterEventMapper) MapToDNSPolicy(obj client.Object) []reconcile.Request {
	return m.mapToPolicyRequest(obj, "dnspolicy", &RefsConfig{})
}

func (m *ClusterEventMapper) mapToPolicyRequest(obj client.Object, policyKind string, policyRefsConfig common.PolicyRefsConfig) []reconcile.Request {
	logger := m.Logger.V(1).WithValues("object", client.ObjectKeyFromObject(obj))

	clusterName := obj.GetName()

	allGwList := &gatewayapiv1beta1.GatewayList{}
	err := m.client.List(context.TODO(), allGwList)
	if err != nil {
		logger.Info("mapToPolicyRequest:", "error", "failed to get gateways")
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)
	for _, gw := range allGwList.Items {
		val := metadata.GetAnnotation(&gw, gateway.ClustersAnnotation)
		if val == "" {
			continue
		}
		var clusters []string
		if err := json.Unmarshal([]byte(val), &clusters); err == nil {
			if slice.ContainsString(clusters, clusterName) {
				requests = append(requests, m.GatewayEventMapper.mapToPolicyRequest(&gw, policyKind, policyRefsConfig)[:]...)
			}
		}
	}

	return requests
}
