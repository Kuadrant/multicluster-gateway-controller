package policysync

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type ResourceEventHandler struct {
	Log           logr.Logger
	GVR           schema.GroupVersionResource
	Client        client.Client
	DynamicClient dynamic.Interface
	Gateway       *gatewayv1beta1.Gateway

	Syncer Syncer
}

var _ cache.ResourceEventHandler = &ResourceEventHandler{}

func (h *ResourceEventHandler) OnAdd(reqObj interface{}, _ bool) {
	h.Log.Info("Got watch event for policy", "obj", reqObj)

	ctx := context.Background()

	obj, ok := reqObj.(client.Object)
	if !ok {
		h.Log.Error(fmt.Errorf("object %v does not inplement client.Object", reqObj), "")
		return
	}

	if err := h.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		h.Log.Error(err, "failed to get object", "object", obj)
	}

	policy, err := NewPolicyFor(obj)
	if err != nil {
		h.Log.Error(err, "failed to build policy from watched object", "object", obj)
		return
	}

	if err := h.Syncer.SyncPolicy(ctx, h.Client, policy); err != nil {
		h.Log.Error(err, "failed to sync policy", "policy", policy)
	}
}

func (h *ResourceEventHandler) OnDelete(obj interface{}) {
	h.Log.Info("Got watch event for policy", "obj", obj)
}

func (h *ResourceEventHandler) OnUpdate(_ interface{}, reqObj interface{}) {
	h.Log.Info("Got watch event for policy", "obj", reqObj)

	ctx := context.Background()

	obj, ok := reqObj.(client.Object)
	if !ok {
		h.Log.Error(fmt.Errorf("object %v does not inplement client.Object", reqObj), "")
		return
	}

	if err := h.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		h.Log.Error(err, "failed to get object", "object", obj)
	}

	policy, err := NewPolicyFor(obj)
	if err != nil {
		h.Log.Error(err, "failed to build policy from watched object", "object", obj)
		return
	}

	if err := h.Syncer.SyncPolicy(ctx, h.Client, policy); err != nil {
		h.Log.Error(err, "failed to sync policy", "policy", policy)
	}
}
