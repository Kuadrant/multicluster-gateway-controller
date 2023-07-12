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
	Queue         *health.QueuedProbeWorker
}

// +kubebuilder:rbac:groups=kuadrant.io,resources=dnshealthcheckprobes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kuadrant.io,resources=dnshealthcheckprobes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kuadrant.io,resources=dnshealthcheckprobes/finalizers,verbs=get;update;patch

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

	protocol = v1alpha1.NewHealthProtocol(string(previous.Spec.Protocol))

	probeId := probeId(previous)

	if r.HealthMonitor.HasProbe(probeId) {
		r.HealthMonitor.UpdateProbe(probeId, func(p *health.ProbeQueuer) {
			p.Interval = interval
			p.Host = previous.Spec.Host
			p.IPAddress = previous.Spec.Address
			p.Path = previous.Spec.Path
			p.Port = previous.Spec.Port
			p.Protocol = protocol
			p.AdditionalHeaders = previous.Spec.AdditionalHeaders
			p.ExpectedReponses = previous.Spec.ExpectedReponses
		})
	} else {
		notifier := NewStatusUpdateProbeNotifier(r.Client, previous)
		r.HealthMonitor.AddProbeQueuer(&health.ProbeQueuer{
			ID:                probeId,
			Interval:          interval,
			Host:              previous.Spec.Host,
			Path:              previous.Spec.Path,
			Port:              previous.Spec.Port,
			Protocol:          protocol,
			IPAddress:         previous.Spec.Address,
			AdditionalHeaders: previous.Spec.AdditionalHeaders,
			ExpectedReponses:  previous.Spec.ExpectedReponses,
			Notifier:          notifier,
			Queue:             r.Queue,
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
