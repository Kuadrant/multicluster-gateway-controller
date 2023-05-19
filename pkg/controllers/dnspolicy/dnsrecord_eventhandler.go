package dnspolicy

import (
	"context"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/gateway"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DNSRecordEventHandler struct {
	client      client.Client
	hostService gateway.HostService
}

var _ handler.EventHandler = &DNSRecordEventHandler{}

// Create implements handler.EventHandler
func (eh *DNSRecordEventHandler) Create(e event.CreateEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.Object, q)
}

// Delete implements handler.EventHandler
func (eh *DNSRecordEventHandler) Delete(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.Object, q)
}

// Generic implements handler.EventHandler
func (eh *DNSRecordEventHandler) Generic(e event.GenericEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.Object, q)
}

// Update implements handler.EventHandler
func (eh *DNSRecordEventHandler) Update(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.ObjectNew, q)
}

func (eh *DNSRecordEventHandler) enqueueForObject(obj v1.Object, q workqueue.RateLimitingInterface) {
	ctx := context.Background()

	dnsRecord, ok := obj.(*v1alpha1.DNSRecord)
	if !ok {
		return
	}

	dnsPolicyList := &v1alpha1.DNSPolicyList{}
	if err := eh.client.List(ctx, dnsPolicyList); err != nil {
		log.Log.Error(err, "unexpected error listing DNSPolicies when enqueuing DNSRecord", "dnsRecord", dnsRecord)
		return
	}

	for _, dnsPolicy := range dnsPolicyList.Items {
		enqueue, err := eh.targets(ctx, &dnsPolicy, dnsRecord)
		if err != nil || !enqueue {
			continue
		}

		q.Add(ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(&dnsPolicy),
		})
	}
}

func (eh *DNSRecordEventHandler) targets(ctx context.Context, policy *v1alpha1.DNSPolicy, dnsRecord *v1alpha1.DNSRecord) (bool, error) {
	dnsRecords, err := getDNSRecords(ctx, eh.client, eh.hostService, policy)
	if err != nil {
		return false, err
	}

	return slice.Contains(dnsRecords, func(targetedRecord *v1alpha1.DNSRecord) bool {
		return targetedRecord.Name == dnsRecord.Name && targetedRecord.Namespace == dnsRecord.Namespace
	}), nil
}
