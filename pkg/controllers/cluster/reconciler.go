package cluster

import (
	"context"
	"strings"
	"time"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/controller"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	IngressName = "mctc-webhooks"
)

type Object struct {
	Name string

	RestConfig *rest.Config
}

type Reconciler interface {
	Reconcile(ctx context.Context, obj Object) (ctrl.Result, error)
}

type AdmissionReconciler struct {
	controlClient client.Client
}

func NewAdmissionReconciler(controlClient client.Client) *AdmissionReconciler {
	return &AdmissionReconciler{controlClient: controlClient}
}

var _ Reconciler = &AdmissionReconciler{}

func (r *AdmissionReconciler) Reconcile(ctx context.Context, obj Object) (ctrl.Result, error) {
	webhookAccessor, ok, err := r.getWebhookTraffic(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// The webhook accessor might have not been reconciled yet, which means
	// the webhook server is not ready to accept requests. Do nothing for now
	if !ok {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Minute}, nil
	}

	// Get the managed host
	managedHosts := webhookAccessor.GetAnnotations()[traffic.AnnotationManagedHosts]
	managedHost := strings.Split(managedHosts, ",")[0]

	tlsSecret, ok, err := r.getTLSSecret(ctx, managedHost, webhookAccessor)
	if err != nil {
		return ctrl.Result{}, nil
	}
	if !ok {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Minute}, nil
	}

	workloadClient, err := r.getWorkloadClient(obj)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Log.Info("getting webhook configurations")
	validatingWebhooks, mutatingWebhooks := webhookAccessor.GetWebhookConfigurations(managedHost, bundleCA(tlsSecret))
	log.Log.Info("create/update validating webhooks")
	for _, webhook := range validatingWebhooks {
		g := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: webhook.ObjectMeta,
		}

		_, err := controllerutil.CreateOrUpdate(ctx, workloadClient, g, func() error {
			g.Webhooks = webhook.Webhooks
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	log.Log.Info("create/update mutating webhooks")
	for _, webhook := range mutatingWebhooks {
		g := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: webhook.ObjectMeta,
		}

		_, err := controllerutil.CreateOrUpdate(ctx, workloadClient, g, func() error {
			g.Webhooks = webhook.Webhooks
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *AdmissionReconciler) getWebhookTraffic(ctx context.Context) (traffic.Interface, bool, error) {
	namespace, ok := controller.GetNamespace()
	if !ok {
		return nil, false, nil
	}

	webhookIngress := &networkingv1.Ingress{}
	err := r.controlClient.Get(ctx, types.NamespacedName{
		Name:      IngressName,
		Namespace: namespace,
	}, webhookIngress)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, err
	}

	if webhookIngress.Annotations == nil || webhookIngress.Annotations[traffic.AnnotationManagedHosts] == "" {
		return nil, false, nil
	}

	if len(webhookIngress.Spec.TLS) == 0 {
		return nil, false, nil
	}

	return traffic.NewIngress(webhookIngress), true, nil
}

func (r *AdmissionReconciler) getTLSSecret(ctx context.Context, managedHost string, webhookAccessor traffic.Interface) (*corev1.Secret, bool, error) {
	tlsConfig, ok := slice.Find(webhookAccessor.GetTLS(), func(t traffic.TLSConfig) bool {
		return slice.ContainsString(t.Hosts, managedHost)
	})
	if !ok {
		return nil, false, nil
	}

	secret := &corev1.Secret{}
	err := r.controlClient.Get(ctx, types.NamespacedName{
		Name:      tlsConfig.SecretName,
		Namespace: webhookAccessor.GetNamespace(),
	}, secret)

	return secret, true, err
}

func (r *AdmissionReconciler) getWorkloadClient(obj Object) (client.Client, error) {
	return client.New(obj.RestConfig, client.Options{})
}

func bundleCA(secret *corev1.Secret) []byte {
	result := []byte{}

	for name, cert := range secret.Data {
		if name == "tls.key" {
			continue
		}

		result = append(result, cert...)
	}

	return result
}
