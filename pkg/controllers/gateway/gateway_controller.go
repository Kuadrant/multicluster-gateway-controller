/*
Copyright 2022 The MultiCluster Traffic Controller Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gateway

import (
	"context"
	"fmt"
	"reflect"
	"time"

	kuadrantapi "github.com/kuadrant/kuadrant-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
)

const (
	ClusterSyncerAnnotation               = "clustersync.kuadrant.io"
	GatewayClusterLabelSelectorAnnotation = "kuadrant.io/gateway-cluster-label-selector"
	GatewayFinalizer                      = "kuadrant.io/gateway"
	ManagedLabel                          = "kuadarant.io/managed"
)

type HostService interface {
	CreateDNSRecord(ctx context.Context, subDomain string, managedZone *v1alpha1.ManagedZone, owner traffic.Interface) (*v1alpha1.DNSRecord, error)
	GetDNSRecord(ctx context.Context, subDomain string, managedZone *v1alpha1.ManagedZone, owner traffic.Interface) (*v1alpha1.DNSRecord, error)
	GetManagedZoneForHost(ctx context.Context, domain string, t traffic.Interface) (*v1alpha1.ManagedZone, string, error)
	SetEndpoints(ctx context.Context, endpoints []gatewayv1beta1.GatewayAddress, dnsRecord *v1alpha1.DNSRecord) error
	GetDNSRecordsFor(ctx context.Context, t traffic.Interface) ([]*v1alpha1.DNSRecord, error)
	CleanupDNSRecords(ctx context.Context, owner traffic.Interface) error
	// GetManagedHosts will return the list of hosts in this gateways listeners that are associated with a managedzone managed by this controller
	GetManagedHosts(ctx context.Context, traffic traffic.Interface) ([]v1alpha1.ManagedHost, error)
}

type CertificateService interface {
	EnsureCertificate(ctx context.Context, host string, owner metav1.Object) error
	GetCertificateSecret(ctx context.Context, host string, namespace string) (*corev1.Secret, error)
	CleanupCertificates(ctx context.Context, owner traffic.Interface) error
}

type GatewayPlacer interface {
	//Place will use the placement logic to create the needed resources and ensure the objects are synced to the targeted clusters
	// it will return the set of clusters it has targeted
	Place(ctx context.Context, upstream *gatewayv1beta1.Gateway, downstream *gatewayv1beta1.Gateway, children ...metav1.Object) (sets.Set[string], error)
	// gets the clusters the gateway has actually been placed on
	GetPlacedClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error)
	//GetClusters returns the clusters decided on by the placement logic
	GetClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error)
	// ListenerTotalAttachedRoutes returns the total attached routes for a listener from the downstream gateways
	ListenerTotalAttachedRoutes(ctx context.Context, gateway *gatewayv1beta1.Gateway, listenerName string, downstream string) (int, error)
	// GetAddresses will look at the downstream view of the gateway and return the LB addresses used for these gateways
	GetAddresses(ctx context.Context, gateway *gatewayv1beta1.Gateway, downstream string) ([]gatewayv1beta1.GatewayAddress, error)
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=placementdecisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Certificates CertificateService
	Host         HostService
	Placement    GatewayPlacer
}

func isDeleting(g *gatewayv1beta1.Gateway) bool {
	return g.GetDeletionTimestamp() != nil && !g.GetDeletionTimestamp().IsZero()
}

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	previous := &gatewayv1beta1.Gateway{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, previous)
	if err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	upstreamGateway := previous.DeepCopy()
	log.V(3).Info("reconciling gateway", "classname", upstreamGateway.Spec.GatewayClassName)
	if isDeleting(upstreamGateway) {
		log.Info("gateway being deleted ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace)
		if _, _, _, err := r.reconcileDownstreamFromUpstreamGateway(ctx, upstreamGateway, nil); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile downstream gateway after upstream gateway deleted: %s ", err)
		}
		fmt.Println("upstream post reconcile ", upstreamGateway.UID, upstreamGateway.ResourceVersion)
		controllerutil.RemoveFinalizer(upstreamGateway, GatewayFinalizer)
		if err := r.Update(ctx, upstreamGateway); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from gateway : %s", err)
		}
		log.Info("gateway being deleted finalizer removed ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace)
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(upstreamGateway, GatewayFinalizer) {
		controllerutil.AddFinalizer(upstreamGateway, GatewayFinalizer)
		if err = r.Update(ctx, upstreamGateway); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer to gateway : %s", err)
		}
		return ctrl.Result{}, nil

	}

	// If the GatewayClass parametrers are invalid, update the status
	// and stop reconciling
	params, err := getParams(ctx, r.Client, string(upstreamGateway.Spec.GatewayClassName))
	if err != nil && IsInvalidParamsError(err) {
		conditions.SetCondition(upstreamGateway.Status.Conditions, previous.Generation, string(gatewayv1beta1.GatewayConditionProgrammed), metav1.ConditionFalse, string(gatewayv1beta1.GatewayReasonPending), fmt.Sprintf("Invalid parameters in gateway class: %s", err.Error()))

		if !reflect.DeepEqual(upstreamGateway, previous) {
			err = r.Status().Update(ctx, upstreamGateway)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}
	if err != nil && !IsInvalidParamsError(err) {
		return ctrl.Result{}, fmt.Errorf("gateway class err %s ", err)
	}
	requeue, programmedStatus, clusters, reconcileErr := r.reconcileDownstreamFromUpstreamGateway(ctx, upstreamGateway, params)
	// gateway now in expected state, place gateway and its associated objects in correct places. Update gateway spec/metadata
	if reconcileErr != nil {
		log.Error(fmt.Errorf("gateway reconcile failed %s", reconcileErr), "gateway failed to reconcile", "gateway", upstreamGateway.Name)
	}
	if reconcileErr == nil && !reflect.DeepEqual(upstreamGateway, previous) {
		return reconcile.Result{}, r.Update(ctx, upstreamGateway)
	}

	upstreamGateway.Status.Conditions = buildAcceptedCondition(upstreamGateway.Status, upstreamGateway.Generation, metav1.ConditionTrue)
	upstreamGateway.Status.Conditions = buildProgrammedStatus(upstreamGateway.Status, upstreamGateway.Generation, clusters, programmedStatus, err)
	if !isDeleting(upstreamGateway) && !reflect.DeepEqual(upstreamGateway.Status, previous.Status) {
		return reconcile.Result{}, r.Status().Update(ctx, upstreamGateway)
	}

	if requeue {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, reconcileErr
	}
	return ctrl.Result{}, reconcileErr
}

// reconcileDownstreamGateway takes the upstream definition and transforms it as needed to apply it to the downstream spokes
func (r *GatewayReconciler) reconcileDownstreamFromUpstreamGateway(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway, params *Params) (bool, metav1.ConditionStatus, []string, error) {
	log := log.FromContext(ctx)
	clusters := []string{}
	downstream := upstreamGateway.DeepCopy()
	downstreamNS := fmt.Sprintf("%s-%s", "kuadrant", downstream.Namespace)
	downstream.Status = gatewayv1beta1.GatewayStatus{}

	// reset this for the sync as we don't want control plane level UID, creation etc etc
	downstream.ObjectMeta = metav1.ObjectMeta{
		Name:        upstreamGateway.Name,
		Namespace:   downstreamNS,
		Labels:      upstreamGateway.Labels,
		Annotations: upstreamGateway.Annotations,
	}
	if downstream.Labels == nil {
		downstream.Labels = map[string]string{}
	}
	downstream.Labels[ManagedLabel] = "true"
	accessor := traffic.NewGateway(upstreamGateway)
	if isDeleting(upstreamGateway) {
		log.Info("deleting downstream gateways owned by upstream gateway ", "name", downstream.Name, "namespace", downstream.Namespace)
		targets, err := r.Placement.Place(ctx, upstreamGateway, downstream)
		if err != nil {
			return false, metav1.ConditionFalse, clusters, err
		}
		log.V(3).Info("cleaning up associated DNSRecords")
		if err := r.Host.CleanupDNSRecords(ctx, accessor); err != nil {
			log.Error(err, "Error deleting DNS record")
			return false, metav1.ConditionFalse, clusters, err
		}

		// Cleanup certificates
		err = r.Certificates.CleanupCertificates(ctx, accessor)
		if err != nil {
			log.Error(err, "Error deleting certs")
			return false, metav1.ConditionFalse, clusters, err
		}
		return false, metav1.ConditionTrue, targets.UnsortedList(), nil
	}

	upstreamAccessor := traffic.NewGateway(upstreamGateway)
	managedHosts, err := r.Host.GetManagedHosts(ctx, upstreamAccessor)
	if err != nil {
		return false, metav1.ConditionFalse, clusters, err
	}
	// ensure tls is set up first before doing anything else. TLS is not affectd by placement changes
	tlsSecrets, err := r.reconcileTLS(ctx, upstreamGateway, downstream, managedHosts)
	if err != nil {
		return true, metav1.ConditionFalse, clusters, err
	}
	log.Info("TLS reconciled for downstream gatway ", "gateway", downstream.Name, "namespace", downstream.Namespace)

	// some of this should be pulled from gateway class params
	if params != nil {
		if err := r.reconcileParams(ctx, downstream, params); err != nil {
			return false, metav1.ConditionUnknown, clusters, err
		}
	}

	// ensure the gatways are placed into the right target clusters and removed from any that are no longer targeted
	targets, err := r.Placement.Place(ctx, upstreamGateway, downstream, tlsSecrets...)
	if err != nil {
		return true, metav1.ConditionFalse, clusters, err
	}

	log.Info("Gateway Placed ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace, "targets", targets.UnsortedList())
	//get updatd list of clusters where this gateway has been successfully placed
	placed, err := r.Placement.GetPlacedClusters(ctx, upstreamGateway)
	if err != nil {
		return false, metav1.ConditionUnknown, targets.UnsortedList(), err
	}
	//update the cluster set
	clusters = placed.UnsortedList()
	//update dns for listeners on the placed gateway
	if err := r.reconcileDNSEndpoints(ctx, upstreamGateway, clusters, managedHosts); err != nil {
		return true, metav1.ConditionFalse, clusters, err
	}
	log.Info("DNS Reconciled ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace)
	if placed.Equal(targets) {
		return false, metav1.ConditionTrue, clusters, nil
	}
	log.Info("Gateway Reconciled Successfully ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace)
	return false, metav1.ConditionUnknown, clusters, nil
}

func (r *GatewayReconciler) reconcileTLS(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway, gateway *gatewayv1beta1.Gateway, managedHosts []v1alpha1.ManagedHost) ([]metav1.Object, error) {
	log := log.FromContext(ctx)
	tlsSecrets := []metav1.Object{}
	accessor := traffic.NewGateway(gateway)

	for _, mh := range managedHosts {
		// Only generate cert for https listeners
		listener := getListenerByHost(gateway, mh.Host)
		if listener == nil || listener.Protocol != gatewayv1beta1.HTTPSProtocolType {
			continue
		}

		// create certificate resource for assigned host
		if err := r.Certificates.EnsureCertificate(ctx, mh.Host, upstreamGateway); err != nil && !k8serrors.IsAlreadyExists(err) {
			return tlsSecrets, err
		}

		// Check if certificate secret is ready
		secret, err := r.Certificates.GetCertificateSecret(ctx, mh.Host, upstreamGateway.Namespace)
		if err != nil && !k8serrors.IsNotFound(err) {
			return tlsSecrets, err
		}
		if err != nil {
			log.V(3).Info("tls secret does not exist yet for host " + mh.Host + " requeue")
			return tlsSecrets, nil
		}

		//sync secret to clusters
		if secret != nil {
			downstreamSecret := secret.DeepCopy()
			downstreamSecret.ObjectMeta = metav1.ObjectMeta{}
			downstreamSecret.Name = secret.Name
			downstreamSecret.Namespace = gateway.Namespace
			downstreamSecret.Labels = secret.Labels
			downstreamSecret.Annotations = secret.Annotations
			accessor.AddTLS(mh.Host, downstreamSecret)
			tlsSecrets = append(tlsSecrets, downstreamSecret)
		}
	}
	return tlsSecrets, nil
}

func (r *GatewayReconciler) reconcileDNSEndpoints(ctx context.Context, gateway *gatewayv1beta1.Gateway, placed []string, managedHosts []v1alpha1.ManagedHost) error {
	log := log.Log
	log.V(3).Info("checking gateway for attached routes ", "gateway", gateway.Name, "clusters", placed, "managed hosts", len(managedHosts))
	if len(placed) == 0 {
		//nothing to do
		log.V(3).Info("reconcileDNSEndpoints gateway has not been placed on to any downstream clusters nothing to do")
		return nil
	}
	for _, mh := range managedHosts {
		endpoints := []gatewayv1beta1.GatewayAddress{}
		for _, downstreamCluster := range placed {
			// Only consider host for dns if there's at least 1 attached route to the listener for this host in *any* gateway
			listener := getListenerByHost(gateway, mh.Host)
			attached, err := r.Placement.ListenerTotalAttachedRoutes(ctx, gateway, string(listener.Name), downstreamCluster)
			if err != nil {
				log.Error(err, "failed to get total attached routes for listener ", "listner", listener.Name)
				continue
			}
			if attached == 0 {
				log.V(3).Info("no attached routes for ", "listner", listener.Name, "cluster ", downstreamCluster)
				continue
			}
			log.V(3).Info("hostHasAttachedRoutes", "host", mh.Host, "hostHasAttachedRoutes", attached)
			addresses, err := r.Placement.GetAddresses(ctx, gateway, downstreamCluster)
			if err != nil {
				return err
			}
			endpoints = append(endpoints, addresses...)
		}

		if len(endpoints) == 0 {
			// delete record
			if mh.DnsRecord != nil {
				if err := r.Delete(ctx, mh.DnsRecord); err != nil {
					return fmt.Errorf("failed to delete dns record %s", err)
				}
			}
			return nil
		}
		gatewayAccessor := traffic.NewGateway(gateway)
		var dnsRecord, err = r.Host.CreateDNSRecord(ctx, mh.Subdomain, mh.ManagedZone, gatewayAccessor)
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return err
		}
		if k8serrors.IsAlreadyExists(err) {
			dnsRecord, err = r.Host.GetDNSRecord(ctx, mh.Subdomain, mh.ManagedZone, gatewayAccessor)
			if err != nil {
				return err
			}
		}

		log.Info("setting dns endpoints for gateway listener", "listener", dnsRecord.Name, "values", endpoints)

		if err := r.Host.SetEndpoints(ctx, endpoints, dnsRecord); err != nil {
			return fmt.Errorf("failed to set dns record endpoints %s %v", err, endpoints)
		}
	}

	return nil
}

func (r *GatewayReconciler) reconcileParams(ctx context.Context, gateway *gatewayv1beta1.Gateway, params *Params) error {

	downstreamClass := params.GetDownstreamClass()

	// Set the annotations to sync the class name from the parameters

	gateway.Spec.GatewayClassName = gatewayv1beta1.ObjectName(downstreamClass)

	return nil
}

func buildProgrammedStatus(gatewayStatus gatewayv1beta1.GatewayStatus, generation int64, placed []string, programmedStatus metav1.ConditionStatus, err error) []metav1.Condition {
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}
	message := "waiting for gateway to placed on clusters %v %s"
	if programmedStatus == metav1.ConditionTrue {
		message = "gateway placed on clusters %v %s"
	}
	if programmedStatus == metav1.ConditionFalse {
		message = "gateway failed to be placed on all clusters %v error %s"
	}
	if programmedStatus == metav1.ConditionUnknown {
		message = "current state of the gateway is unknown error %s"
	}
	return conditions.SetCondition(gatewayStatus.Conditions, generation, string(gatewayv1beta1.GatewayConditionProgrammed), programmedStatus, string(gatewayv1beta1.GatewayReasonProgrammed), fmt.Sprintf(message, placed, errorMsg))

}

func buildAcceptedCondition(gatewayStatus gatewayv1beta1.GatewayStatus, generation int64, acceptedStatus metav1.ConditionStatus) []metav1.Condition {
	statusConditions := []metav1.Condition{}
	message := fmt.Sprintf("Handled by %s", ControllerName)

	// State has changed
	return conditions.SetCondition(statusConditions, generation, string(gatewayv1beta1.GatewayConditionAccepted), metav1.ConditionTrue, string(gatewayv1beta1.GatewayConditionAccepted), message)

}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {
	log := log.FromContext(ctx)
	//TODO need to trigger gatway reconcile when gatewayclass params changes
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.Gateway{}).
		Watches(&source.Kind{Type: &workv1.ManifestWork{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			log.V(3).Info("enqueuing gateways based on manifest work change ", "work namespace", o.GetNamespace())
			requests := []reconcile.Request{}
			annotations := o.GetAnnotations()
			if annotations == nil {
				log.V(3).Info("no parent or anotations on manifest work ", "work ns", o.GetNamespace(), "name", o.GetName())
				return requests
			}
			key := annotations["kuadrant.io/parent"]
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				log.Error(err, "failed to parse namespace and name from maifiest work")
				return requests
			}
			log.Info("requeuing gateway ", "namespace", ns, "name", name)
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
			})
			return requests
		}), builder.OnlyMetadata).
		Watches(&source.Kind{Type: &clusterv1beta2.PlacementDecision{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			// kinda want to get the old and new object here and only queue if the clusters have changed
			// queue up gateways in this namespace
			log.V(3).Info("enqueuing gateways based on placementdecision change ", " namespace", o.GetNamespace())
			req := []reconcile.Request{}
			l := &gatewayv1beta1.GatewayList{}
			if err := mgr.GetClient().List(ctx, l, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
				log.Error(err, "failed to list gateways to requeue")
				return req
			}
			for _, g := range l.Items {
				req = append(req, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&g),
				})
			}
			return req
		})).
		Watches(&source.Kind{
			Type: &corev1.Secret{},
		}, &ClusterEventHandler{client: r.Client}).
		Watches(&source.Kind{Type: &workv1.ManifestWork{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			log.V(3).Info("enqueuing gateways based on manifest work change ", "work namespace", o.GetNamespace())
			requests := []reconcile.Request{}
			annotations := o.GetAnnotations()
			if annotations == nil {
				log.V(3).Info("no parent or anotations on manifest work ", "work ns", o.GetNamespace(), "name", o.GetName())
				return requests
			}
			key := annotations["kuadrant.io/parent"]
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				log.Error(err, "failed to parse namespace and name from maifiest work")
				return requests
			}
			log.Info("requeuing gateway ", "namespace", ns, "name", name)
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
			})
			return requests
		}), builder.OnlyMetadata).
		Watches(&source.Kind{Type: &clusterv1beta2.PlacementDecision{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
			// kinda want to get the old and new object here and only queue if the clusters have changed
			// queue up gateways in this namespace
			req := []reconcile.Request{}
			l := &gatewayv1beta1.GatewayList{}
			if err := mgr.GetClient().List(ctx, l, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
				log.Error(err, "failed to list gateways to requeue")
				return req
			}
			for _, g := range l.Items {
				req = append(req, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&g),
				})
			}
			return req
		})).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			gateway, ok := object.(*gatewayv1beta1.Gateway)
			if ok {
				shouldReconcile := slice.ContainsString(getSupportedClasses(), string(gateway.Spec.GatewayClassName))
				log.V(3).Info(" should reconcile", "gateway", gateway.Name, "with class ", gateway.Spec.GatewayClassName, "should ", shouldReconcile)
				return slice.ContainsString(getSupportedClasses(), string(gateway.Spec.GatewayClassName))
			}
			return true
		})).
		Owns(&kuadrantapi.RateLimitPolicy{}).
		Complete(r)
}

// This is very specific to gateways so not in traffic interface
func getListenerByHost(g *gatewayv1beta1.Gateway, host string) *gatewayv1beta1.Listener {
	for _, listener := range g.Spec.Listeners {
		if *(*string)(listener.Hostname) == host {
			return &listener
		}
	}
	return nil
}
