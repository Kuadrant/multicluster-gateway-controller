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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/gracePeriod"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/policy"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/tls"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
)

const (
	GatewayClusterLabelSelectorAnnotation                            = "kuadrant.io/gateway-cluster-label-selector"
	GatewayClustersAnnotation                                        = "kuadrant.io/gateway-clusters"
	GatewayFinalizer                                                 = "kuadrant.io/gateway"
	ManagedLabel                                                     = "kuadrant.io/managed"
	MultiClusterIPAddressType             gatewayv1beta1.AddressType = "kuadrant.io/MultiClusterIPAddress"
	MultiClusterHostnameAddressType       gatewayv1beta1.AddressType = "kuadrant.io/MultiClusterHostnameAddress"
)

type CertificateService interface {
	EnsureCertificate(ctx context.Context, name, host string, owner metav1.Object) error
	GetCertificateSecret(ctx context.Context, name string, namespace string) (*corev1.Secret, error)
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
	// GetClusterGateway
	GetClusterGateway(ctx context.Context, gateway *gatewayv1beta1.Gateway, clusterName string) (dns.ClusterGateway, error)
}

var ReconcileErrTLS = fmt.Errorf("failed to reconcile TLS")

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=placementdecisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Certificates CertificateService
	Placement    GatewayPlacer
}

func isDeleting(g *gatewayv1beta1.Gateway) bool {
	return g.GetDeletionTimestamp() != nil && !g.GetDeletionTimestamp().IsZero()
}

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	previous := &gatewayv1beta1.Gateway{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, previous)
	if err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	fmt.Println("previous", previous.ObjectMeta, previous.Spec, previous.Status)
	upstreamGateway := previous.DeepCopy()
	log.V(3).Info("reconciling gateway", "classname", upstreamGateway.Spec.GatewayClassName)
	if isDeleting(upstreamGateway) {
		log.Info("gateway being deleted ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace)
		if _, _, _, err := r.reconcileDownstreamFromUpstreamGateway(ctx, upstreamGateway, nil); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile downstream gateway after upstream gateway deleted: %s ", err)
		}
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

	// If the GatewayClass parameters are invalid, update the status and stop reconciling
	params, err := getParams(ctx, r.Client, string(upstreamGateway.Spec.GatewayClassName))
	if err != nil && IsInvalidParamsError(err) {
		programmedCondition := metav1.Condition{
			Type:               string(gatewayv1beta1.GatewayConditionProgrammed),
			Status:             metav1.ConditionFalse,
			Reason:             string(gatewayv1beta1.GatewayReasonPending),
			Message:            fmt.Sprintf("Invalid parameters in gateway class: %s", err.Error()),
			ObservedGeneration: previous.Generation,
		}
		meta.SetStatusCondition(&upstreamGateway.Status.Conditions, programmedCondition)

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
	//if we get to the point where we are going to reconcile the gateway in to the downstream then the upstream gateway is considered accepted
	// checking not true covers "unknown" and false
	if !meta.IsStatusConditionTrue(upstreamGateway.Status.Conditions, string(gatewayv1beta1.GatewayConditionAccepted)) {
		log.V(3).Info("gateway is accepted setting initial programmed and accepted status")
		acceptedCondition := buildAcceptedCondition(upstreamGateway.Generation, metav1.ConditionTrue)
		programmedCondition := buildProgrammedCondition(upstreamGateway.Generation, []string{}, metav1.ConditionUnknown, nil)
		meta.SetStatusCondition(&upstreamGateway.Status.Conditions, acceptedCondition)
		meta.SetStatusCondition(&upstreamGateway.Status.Conditions, programmedCondition)
		return reconcile.Result{}, r.Status().Update(ctx, upstreamGateway)
	}

	log.V(3).Info("gateway pre downstream", "labels", upstreamGateway.Labels)
	requeue, programmedStatus, clusters, reconcileErr := r.reconcileDownstreamFromUpstreamGateway(ctx, upstreamGateway, params)
	log.V(3).Info("gateway post downstream", "labels", upstreamGateway.Labels)
	// gateway now in expected state, place gateway and its associated objects in correct places. Update gateway spec/metadata
	log.V(3).Info("reconcileDownstreamFromUpstreamGateway result ", "requeue", requeue, "status", programmedStatus, "clusters", clusters, "Err", reconcileErr)
	if reconcileErr != nil {
		if errors.Is(reconcileErr, gracePeriod.ErrGracePeriodNotExpired) || requeue {
			log.V(3).Info("requeueing gateway ", "error", reconcileErr, "requeue", requeue)
			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: 30 * time.Second,
			}, nil
		}
		log.Error(fmt.Errorf("gateway reconcile failed %s", reconcileErr), "gateway failed to reconcile", "gateway", upstreamGateway.Name)
	}

	serialized, err := json.Marshal(clusters)
	if err != nil {
		return ctrl.Result{}, err
	}
	metadata.AddAnnotation(upstreamGateway, GatewayClustersAnnotation, string(serialized))

	if reconcileErr == nil && !reflect.DeepEqual(upstreamGateway, previous) {
		log.Info("updating upstream gateway")
		return reconcile.Result{}, r.Update(ctx, upstreamGateway)
	}

	var addressErr error
	allAddresses := []gatewayv1beta1.GatewayAddress{}
	for _, cluster := range clusters {
		log.V(3).Info("checking cluster for addresses", "cluster", cluster)
		addresses, addressErr := r.Placement.GetAddresses(ctx, upstreamGateway, cluster)
		log.V(3).Info("got addresses", "addresses,", addresses, "addressErr", addressErr)
		if addressErr != nil {
			break
		}
		for _, address := range addresses {
			log.V(3).Info("checking address type for mapping", "address.Type", address.Type)
			var addressType gatewayv1beta1.AddressType
			if *address.Type == gatewayv1beta1.IPAddressType {
				addressType = MultiClusterIPAddressType
			} else if *address.Type == gatewayv1beta1.HostnameAddressType {
				addressType = MultiClusterHostnameAddressType
			} else {
				break // ignore address type. Unsupported for multi cluster gateway
			}
			allAddresses = append(allAddresses, gatewayv1beta1.GatewayAddress{
				Type:  &addressType,
				Value: fmt.Sprintf("%s/%s", cluster, address.Value),
			})
		}
	}
	if addressErr != nil {
		return ctrl.Result{}, reconcileErr
	}
	log.V(3).Info("allAddresses", "allAddresses", allAddresses)
	upstreamGateway.Status.Addresses = allAddresses

	allListenerStatuses := []gatewayv1beta1.ListenerStatus{}
	specListeners := upstreamGateway.Spec.Listeners
	for _, listener := range specListeners {
		for _, cluster := range clusters {
			attachedRoutes, err := r.Placement.ListenerTotalAttachedRoutes(ctx, upstreamGateway, string(listener.Name), cluster)
			if err != nil {
				// May not have the status yet, let's ignore, but output info logs about it
				log.Info("AttachedRoutes unknown for listener. Ignoring", "listener", listener.Name, "cluster", cluster, "message", err)
				continue
			}
			allListenerStatuses = append(allListenerStatuses, gatewayv1beta1.ListenerStatus{
				Name:           gatewayv1beta1.SectionName(fmt.Sprintf("%s.%s", cluster, string(listener.Name))),
				AttachedRoutes: int32(attachedRoutes),
				SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
				Conditions:     []metav1.Condition{},
			})
		}
	}
	upstreamGateway.Status.Listeners = allListenerStatuses

	acceptedCondition := buildAcceptedCondition(upstreamGateway.Generation, metav1.ConditionTrue)
	programmedCondition := buildProgrammedCondition(upstreamGateway.Generation, clusters, programmedStatus, err)

	meta.SetStatusCondition(&upstreamGateway.Status.Conditions, acceptedCondition)
	meta.SetStatusCondition(&upstreamGateway.Status.Conditions, programmedCondition)

	if !isDeleting(upstreamGateway) && !reflect.DeepEqual(upstreamGateway.Status, previous.Status) {
		return reconcile.Result{}, r.Status().Update(ctx, upstreamGateway)
	}

	if requeue {
		log.V(3).Info("requeuing gateay in ", "namespace", upstreamGateway.Namespace, "with name", upstreamGateway.Name)
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, reconcileErr
	}
	return ctrl.Result{}, reconcileErr
}

// reconcileDownstreamGateway takes the upstream definition and transforms it as needed to apply it to the downstream spokes
func (r *GatewayReconciler) reconcileDownstreamFromUpstreamGateway(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway, params *Params) (bool, metav1.ConditionStatus, []string, error) {
	fmt.Println("downstream from upstream")
	log := crlog.FromContext(ctx)
	clusters := []string{}
	downstream := upstreamGateway.DeepCopy()
	downstreamNS := fmt.Sprintf("%s-%s", "kuadrant", downstream.Namespace)
	downstream.Status = gatewayv1beta1.GatewayStatus{}

	// reset this for the sync as we don't want control plane level UID, creation etc etc
	downstream.ObjectMeta = metav1.ObjectMeta{
		Name:        upstreamGateway.Name,
		Namespace:   downstreamNS,
		Labels:      downstream.Labels,
		Annotations: downstream.Annotations,
	}
	if downstream.Labels == nil {
		downstream.Labels = map[string]string{}
	}
	downstream.Labels[ManagedLabel] = "true"
	if isDeleting(upstreamGateway) {
		log.Info("deleting downstream gateways owned by upstream gateway ", "name", downstream.Name, "namespace", downstream.Namespace)
		targets, err := r.Placement.Place(ctx, upstreamGateway, downstream)
		if err != nil {
			return false, metav1.ConditionFalse, clusters, err
		}
		return false, metav1.ConditionTrue, targets.UnsortedList(), nil
	}

	if len(upstreamGateway.Spec.Listeners) == 0 {
		return false, metav1.ConditionFalse, clusters, fmt.Errorf("no managed listeners found")
	}

	// ensure tls is set up first before doing anything else. TLS is not affected by placement changes
	tlsSecrets, err := r.reconcileTLS(ctx, upstreamGateway, downstream)
	if err != nil {
		log.Info("TLS is not ready for downstream gateway ", "gateway", downstream.Name, "namespace", downstream.Namespace, "err", err)
		return true, metav1.ConditionFalse, clusters, errors.Join(ReconcileErrTLS, err)
	}
	log.Info("TLS reconciled for downstream gateway ", "gateway", downstream.Name, "namespace", downstream.Namespace)

	// some of this should be pulled from gateway class params
	if params != nil {
		if err := r.reconcileParams(ctx, downstream, params); err != nil {
			return false, metav1.ConditionUnknown, clusters, fmt.Errorf("failed to get reconcileParams : %s", err)
		}
	}

	// ensure the gateways are placed into the right target clusters and removed from any that are no longer targeted
	targets, err := r.Placement.Place(ctx, upstreamGateway, downstream, tlsSecrets...)
	if err != nil {
		return true, metav1.ConditionFalse, clusters, fmt.Errorf("failed to place gateway : %w", err)
	}

	log.Info("Gateway Placed ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace, "targets", targets.UnsortedList())
	//get updatd list of clusters where this gateway has been successfully placed
	placed, err := r.Placement.GetPlacedClusters(ctx, upstreamGateway)
	if err != nil {
		return false, metav1.ConditionUnknown, targets.UnsortedList(), fmt.Errorf("failed to get placed clusters : %s", err)
	}
	//update the cluster set, needs to be ordered or the status update can continually change and cause spurious updates
	clusters = sets.List(placed)
	if placed.Equal(targets) && placed.Len() > 0 {
		return false, metav1.ConditionTrue, clusters, nil
	}
	log.Info("Gateway Reconciled Successfully ", "gateway", upstreamGateway.Name, "namespace", upstreamGateway.Namespace)
	return false, metav1.ConditionUnknown, clusters, nil
}

func (r *GatewayReconciler) reconcileTLS(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway, gateway *gatewayv1beta1.Gateway) ([]metav1.Object, error) {
	log := crlog.FromContext(ctx)
	tlsSecrets := []metav1.Object{}
	accessor := traffic.NewGateway(gateway)
	for _, listener := range upstreamGateway.Spec.Listeners {
		host := string(*listener.Hostname)
		if host == "" {
			log.Info("skipping listener with no host", listener.Name, "in namespace", gateway.Namespace)
			continue
		}

		if listener.Protocol != gatewayv1beta1.HTTPSProtocolType {
			continue
		}
		certName := certname(upstreamGateway.Name, string(listener.Name))
		// create certificate resource for assigned host
		if err := r.Certificates.EnsureCertificate(ctx, certName, host, upstreamGateway); err != nil && !k8serrors.IsAlreadyExists(err) {
			return tlsSecrets, err
		}

		// Check if certificate secret is ready
		secret, err := r.Certificates.GetCertificateSecret(ctx, certName, upstreamGateway.Namespace)
		if err != nil {
			return tlsSecrets, err
		}

		//sync secret to clusters
		if secret != nil {
			downstreamSecret := secret.DeepCopy()
			downstreamSecret.ObjectMeta = metav1.ObjectMeta{}
			downstreamSecret.Name = secret.Name
			downstreamSecret.Namespace = gateway.Namespace
			downstreamSecret.Labels = secret.Labels
			downstreamSecret.Annotations = secret.Annotations
			accessor.AddTLS(host, downstreamSecret)
			tlsSecrets = append(tlsSecrets, downstreamSecret)
		}

	}
	// ensure only certificates for active listeners are in place not this logic will move to a TLSPolicy controller in the future
	labelSelector := &client.MatchingLabels{
		tls.TLSGatewayOwnerLabel: string(upstreamGateway.GetUID()),
	}
	certList := &certman.CertificateList{}
	if err := r.List(ctx, certList, labelSelector); err != nil {
		return tlsSecrets, err
	}
	for _, cert := range certList.Items {
		validCert := false
		for _, listener := range upstreamGateway.Spec.Listeners {
			if cert.Name == certname(upstreamGateway.Name, string(listener.Name)) {
				validCert = true
				break
			}
		}
		if !validCert {
			if err := r.Delete(ctx, &cert, &client.DeleteOptions{}); err != nil {
				return tlsSecrets, err
			}
		}
	}
	return tlsSecrets, nil
}

func certname(gatwayName, listenerName string) string {
	return fmt.Sprintf("%s-%s", gatwayName, listenerName)
}

func (r *GatewayReconciler) reconcileParams(_ context.Context, gateway *gatewayv1beta1.Gateway, params *Params) error {

	downstreamClass := params.GetDownstreamClass()

	// Set the annotations to sync the class name from the parameters

	gateway.Spec.GatewayClassName = gatewayv1beta1.ObjectName(downstreamClass)

	return nil
}

func buildProgrammedCondition(generation int64, placed []string, programmedStatus metav1.ConditionStatus, err error) metav1.Condition {
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

	cond := metav1.Condition{
		Type:               string(gatewayv1beta1.GatewayConditionProgrammed),
		Status:             programmedStatus,
		Reason:             string(gatewayv1beta1.GatewayReasonProgrammed),
		Message:            fmt.Sprintf(message, placed, errorMsg),
		ObservedGeneration: generation,
	}
	return cond
}

func buildAcceptedCondition(generation int64, acceptedStatus metav1.ConditionStatus) metav1.Condition {
	cond := metav1.Condition{
		Type:               string(gatewayv1beta1.GatewayConditionAccepted),
		Status:             acceptedStatus,
		Reason:             string(gatewayv1beta1.GatewayReasonAccepted),
		Message:            fmt.Sprintf("Handled by %s", ControllerName),
		ObservedGeneration: generation,
	}
	return cond
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {
	log := crlog.FromContext(ctx)

	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1alpha1.DNSPolicy{},
		policy.POLICY_TARGET_REF_KEY,
		func(obj client.Object) []string {
			return []string{policy.GetTargetRefValueFromPolicy(obj.(*v1alpha1.DNSPolicy))}
		},
	)
	if err != nil {
		return err
	}

	//TODO need to trigger gateway reconcile when gatewayclass params changes
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
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			gateway, ok := object.(*gatewayv1beta1.Gateway)
			if ok {
				shouldReconcile := slice.ContainsString(getSupportedClasses(), string(gateway.Spec.GatewayClassName))
				log.V(3).Info(" should reconcile", "gateway", gateway.Name, "with class ", gateway.Spec.GatewayClassName, "should ", shouldReconcile)
				return slice.ContainsString(getSupportedClasses(), string(gateway.Spec.GatewayClassName))
			}
			return true
		})).
		Complete(r)
}
