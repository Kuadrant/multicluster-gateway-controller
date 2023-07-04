package dnshealthcheckprobe

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/health"
)

const (
	DNSHealthCheckProbeFinalizer = "kuadrant.io/dns-health-check-probe"
)

type DNSHealthCheckProbeReconciler struct {
	client.Client
	HealthMonitor *health.Monitor
	Queue         *health.RequestQueue
}

func (r *DNSHealthCheckProbeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	previous := &v1alpha1.DNSHealthCheckProbe{}
	err := r.Client.Get(ctx, req.NamespacedName, previous)
	if err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	logger.V(3).Info("DNSHealthCheckProbeReconciler Reconcile", "dnsHealthCheckProbe", previous)

	probeObj := previous.DeepCopy()

	if probeObj.DeletionTimestamp != nil && !probeObj.DeletionTimestamp.IsZero() {
		logger.Info("deleting probe", "probe", probeObj)

		r.deleteProbe(probeObj)
		controllerutil.RemoveFinalizer(probeObj, DNSHealthCheckProbeFinalizer)

		if err := r.Update(ctx, probeObj); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(probeObj, DNSHealthCheckProbeFinalizer) {
		controllerutil.AddFinalizer(probeObj, DNSHealthCheckProbeFinalizer)
		if err := r.Update(ctx, probeObj); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set the interval
	interval := previous.Spec.Interval.Duration

	// Set the protocol: default to HTTP is not defined
	protocol := previous.Spec.Protocol
	if protocol == "" {
		protocol = v1alpha1.HttpProtocol
	}

	probeId := probeId(previous)

	if r.HealthMonitor.HasProbe(probeId) {
		r.HealthMonitor.UpdateProbe(probeId, func(p *health.Probe) {
			p.Interval = interval
			p.Host = previous.Spec.Host
			p.IPAddress = previous.Spec.IPAddress
			p.Path = previous.Spec.Path
			p.Port = previous.Spec.Port
			p.Protocol = protocol
		})
	} else {
		notifier := NewStatusUpdateProbeNotifier(r.Client, previous)
		r.HealthMonitor.AddProbe(&health.Probe{
			ID:        probeId,
			Interval:  interval,
			Host:      previous.Spec.Host,
			Path:      previous.Spec.Path,
			Port:      previous.Spec.Port,
			Protocol:  protocol,
			IPAddress: previous.Spec.IPAddress,
			Notifier:  notifier,
			Queue:     r.Queue,
		})
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the manager
func (r *DNSHealthCheckProbeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DNSHealthCheckProbe{}).
		Complete(r)
}

func (r *DNSHealthCheckProbeReconciler) deleteProbe(probeObj *v1alpha1.DNSHealthCheckProbe) {
	r.HealthMonitor.RemoveProbe(probeId(probeObj))
}

func probeId(probeObj *v1alpha1.DNSHealthCheckProbe) string {
	return fmt.Sprintf("%s/%s", probeObj.Namespace, probeObj.Name)
}
