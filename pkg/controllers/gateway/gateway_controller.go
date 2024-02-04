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
	"strings"
	"time"

	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/gracePeriod"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/policysync"
)

const (
	LabelPrefix                           = "kuadrant.io/"
	GatewayClusterLabelSelectorAnnotation = LabelPrefix + "gateway-cluster-label-selector"
	GatewayClustersAnnotation             = LabelPrefix + "gateway-clusters"
	GatewayFinalizer                      = LabelPrefix + "gateway"
	ManagedLabel                          = LabelPrefix + "managed"
)

type GatewayPlacer interface {
	//Place will use the placement logic to create the needed resources and ensure the objects are synced to the targeted clusters
	// it will return the set of clusters it has targeted
	Place(ctx context.Context, upstream *gatewayapiv1.Gateway, downstream *gatewayapiv1.Gateway, children ...metav1.Object) (sets.Set[string], error)
	// gets the clusters the gateway has actually been placed on
	GetPlacedClusters(ctx context.Context, gateway *gatewayapiv1.Gateway) (sets.Set[string], error)
	//GetClusters returns the clusters decided on by the placement logic
	GetClusters(ctx context.Context, gateway *gatewayapiv1.Gateway) (sets.Set[string], error)
	// ListenerTotalAttachedRoutes returns the total attached routes for a listener from the downstream gateways
	ListenerTotalAttachedRoutes(ctx context.Context, gateway *gatewayapiv1.Gateway, listenerName string, downstream string) (int, error)
	// GetAddresses will look at the downstream view of the gateway and return the LB addresses used for these gateways
	GetAddresses(ctx context.Context, gateway *gatewayapiv1.Gateway, downstream string) ([]gatewayapiv1.GatewayAddress, error)
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=placementdecisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch

// +kubebuilder:rbac:groups="kuadrant.io",resources=authpolicies;ratelimitpolicies,verbs=get;list;watch

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme                 *runtime.Scheme
	Placement              GatewayPlacer
	PolicyInformersManager *policysync.PolicyInformersManager
	DynamicClient          dynamic.Interface
	WatchedPolicies        map[schema.GroupVersionResource]cache.ResourceEventHandlerRegistration
}

func isDeleting(g *gatewayapiv1.Gateway) bool {
	return g.GetDeletionTimestamp() != nil && !g.GetDeletionTimestamp().IsZero()
}

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	previous := &gatewayapiv1.Gateway{}
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
		if _, _, _, err := r.reconcileDownstreamFromUpstreamGateway(ctx, upstreamGateway, nil); client.IgnoreNotFound(err) != nil {
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
			Type:               string(gatewayapiv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionFalse,
			Reason:             string(gatewayapiv1.GatewayReasonPending),
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
	if !meta.IsStatusConditionTrue(upstreamGateway.Status.Conditions, string(gatewayapiv1.GatewayConditionAccepted)) {
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
		//TODO (cbrookes) refactor how status is handled in this controller
		if errors.Is(reconcileErr, gracePeriod.ErrGracePeriodNotExpired) || requeue {
			log.V(3).Info("requeueing gateway ", "error", reconcileErr, "requeue", requeue)
			programmedCondition := buildProgrammedCondition(upstreamGateway.Generation, clusters, metav1.ConditionUnknown, reconcileErr)
			meta.SetStatusCondition(&upstreamGateway.Status.Conditions, programmedCondition)
			if !isDeleting(upstreamGateway) && !reflect.DeepEqual(upstreamGateway.Status, previous.Status) {
				return reconcile.Result{}, r.Status().Update(ctx, upstreamGateway)
			}
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

	// Map cluster labels onto the gateway
	err = r.reconcileClusterLabels(ctx, upstreamGateway, clusters)
	if err != nil {
		return ctrl.Result{}, err
	}

	if reconcileErr == nil && !reflect.DeepEqual(upstreamGateway, previous) {
		log.Info("updating upstream gateway")
		return reconcile.Result{}, r.Update(ctx, upstreamGateway)
	}

	var addressErr error
	allAddresses := []gatewayapiv1.GatewayStatusAddress{}
	for _, cluster := range clusters {
		log.V(3).Info("checking cluster for addresses", "cluster", cluster)
		addresses, addressErr := r.Placement.GetAddresses(ctx, upstreamGateway, cluster)
		log.V(3).Info("got addresses", "addresses,", addresses, "addressErr", addressErr)
		if addressErr != nil {
			break
		}
		for _, address := range addresses {
			log.V(3).Info("checking address type for mapping", "address.Type", address.Type)
			addressType, supported := AddressTypeToMultiCluster(address)
			if !supported {
				continue // ignore address type gatewayapiv1.NamedAddressType. Unsupported for multi cluster gateway
			}
			allAddresses = append(allAddresses, gatewayapiv1.GatewayStatusAddress{
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

	allListenerStatuses := []gatewayapiv1.ListenerStatus{}
	specListeners := upstreamGateway.Spec.Listeners
	for _, listener := range specListeners {
		for _, cluster := range clusters {
			attachedRoutes, err := r.Placement.ListenerTotalAttachedRoutes(ctx, upstreamGateway, string(listener.Name), cluster)
			if err != nil {
				// May not have the status yet, let's ignore, but output info logs about it
				log.Info("AttachedRoutes unknown for listener. Ignoring", "listener", listener.Name, "cluster", cluster, "message", err)
				continue
			}
			allListenerStatuses = append(allListenerStatuses, gatewayapiv1.ListenerStatus{
				Name:           gatewayapiv1.SectionName(fmt.Sprintf("%s.%s", cluster, string(listener.Name))),
				AttachedRoutes: int32(attachedRoutes),
				SupportedKinds: []gatewayapiv1.RouteGroupKind{},
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
		log.V(3).Info("requeuing gateway in ", "namespace", upstreamGateway.Namespace, "with name", upstreamGateway.Name)
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, reconcileErr
	}
	return ctrl.Result{}, reconcileErr
}

// reconcileClusterLabels fetches labels from ManagedCluster related to clusters array and adds them to the provided Gateway
func (r *GatewayReconciler) reconcileClusterLabels(ctx context.Context, gateway *gatewayapiv1.Gateway, clusters []string) error {
	//Remove all existing clusters.kuadrant.io labels
	for key := range gateway.Labels {
		if strings.HasPrefix(key, ClustersLabelPrefix) {
			delete(gateway.Labels, key)
		}
	}

	//Add clusters.kuadrant.io labels for current clusters
	for _, cluster := range clusters {
		managedCluster := &clusterv1.ManagedCluster{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: cluster}, managedCluster); client.IgnoreNotFound(err) != nil {
			return err
		}

		for key, value := range managedCluster.Labels {
			attribute, found := strings.CutPrefix(key, LabelPrefix)
			if !found {
				continue
			}
			gateway.Labels[ClustersLabelPrefix+cluster+"_"+attribute] = value
		}
	}
	return nil
}

// reconcileDownstreamGateway takes the upstream definition and transforms it as needed to apply it to the downstream spokes
func (r *GatewayReconciler) reconcileDownstreamFromUpstreamGateway(ctx context.Context, upstreamGateway *gatewayapiv1.Gateway, params *Params) (bool, metav1.ConditionStatus, []string, error) {
	log := crlog.FromContext(ctx)
	clusters := []string{}
	downstream := upstreamGateway.DeepCopy()
	downstreamNS := fmt.Sprintf("%s-%s", "kuadrant", downstream.Namespace)
	downstream.Status = gatewayapiv1.GatewayStatus{}

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

	// get tls secrets for all TLS listeners.
	tlsSecrets, err := r.getTLSSecrets(ctx, upstreamGateway, downstream)
	if err != nil {
		return true, metav1.ConditionFalse, clusters, fmt.Errorf("failed to get tls secrets : %s", err)
	}

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
	//get updated list of clusters where this gateway has been successfully placed
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

func (r *GatewayReconciler) getTLSSecrets(ctx context.Context, upstreamGateway *gatewayapiv1.Gateway, downstreamGateway *gatewayapiv1.Gateway) ([]metav1.Object, error) {
	log := crlog.FromContext(ctx)
	tlsSecrets := []metav1.Object{}
	var listenerTLSErr error
	for _, listener := range upstreamGateway.Spec.Listeners {
		if listener.TLS != nil {
			for _, secretRef := range listener.TLS.CertificateRefs {
				ns := upstreamGateway.GetNamespace()
				if secretRef.Namespace != nil {
					ns = string(*secretRef.Namespace)
				}
				tlsSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      string(secretRef.Name),
					Namespace: ns,
				}}
				if err := r.Client.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
					log.Error(err, "cant find tls secret")
					listenerTLSErr = errors.Join(fmt.Errorf("failed to find tls secret for listener %s %w", listener.Name, err))
					continue
				}

				downstreamSecret := tlsSecret.DeepCopy()
				downstreamSecret.ObjectMeta = metav1.ObjectMeta{}
				downstreamSecret.Name = tlsSecret.Name
				downstreamSecret.Namespace = downstreamGateway.Namespace
				downstreamSecret.Labels = tlsSecret.Labels
				downstreamSecret.Annotations = tlsSecret.Annotations

				tlsSecrets = append(tlsSecrets, downstreamSecret)
			}
		}
	}
	return tlsSecrets, listenerTLSErr
}

func (r *GatewayReconciler) reconcileParams(ctx context.Context, gateway *gatewayapiv1.Gateway, params *Params) error {
	log := crlog.FromContext(ctx)

	downstreamClass := params.GetDownstreamClass()

	// Set the annotations to sync the class name from the parameters

	gateway.Spec.GatewayClassName = gatewayapiv1.ObjectName(downstreamClass)

	policiesToSync := slice.Map(params.PoliciesToSync, ParamsGroupVersionResource.ToGroupVersionResource)

	for _, gvr := range policiesToSync {
		// If it's already watched skip it
		_, ok := r.WatchedPolicies[gvr]
		if ok {
			continue
		}

		log.Info("Creating event handler for policy", "gvr", gvr)

		// Add the event handler for the policy
		eventHandler := &policysync.ResourceEventHandler{
			Log:           log,
			GVR:           gvr,
			Client:        r.Client,
			DynamicClient: r.DynamicClient,
			Gateway:       gateway,
			Syncer:        &policysync.FakeSyncer{},
		}
		informer := r.PolicyInformersManager.InformerFactory.ForResource(gvr).Informer()
		reg, err := informer.AddEventHandler(eventHandler)
		if err != nil {
			return err
		}

		// Start the informer
		if err := r.PolicyInformersManager.AddInformer(informer); err != nil {
			return err
		}

		// Keep track of the watched policy
		r.WatchedPolicies[gvr] = reg
	}

	// Stop watching policies if they're removed from the params
	policiesToUnwatch := []schema.GroupVersionResource{}
	for gvr, reg := range r.WatchedPolicies {
		if slice.Contains(policiesToSync, slice.EqualsTo(gvr)) {
			continue
		}

		log.Info("Stopping watch for policy", "gvr", gvr)

		if err := r.PolicyInformersManager.InformerFactory.ForResource(gvr).Informer().RemoveEventHandler(reg); err != nil {
			return err
		}

		policiesToUnwatch = append(policiesToUnwatch, gvr)
	}

	for _, gvr := range policiesToUnwatch {
		delete(r.WatchedPolicies, gvr)
	}

	return nil
}

func buildProgrammedCondition(generation int64, placed []string, programmedStatus metav1.ConditionStatus, err error) metav1.Condition {
	var reason = gatewayapiv1.GatewayReasonProgrammed
	message := "waiting for gateway to placed on clusters %v"
	if programmedStatus == metav1.ConditionTrue {
		message = fmt.Sprintf("gateway placed on clusters %v", placed)
	}
	if programmedStatus == metav1.ConditionFalse {
		message = fmt.Sprintf("gateway failed to be placed on all clusters %v", placed)
		reason = gatewayapiv1.GatewayReasonInvalid
	}
	if programmedStatus == metav1.ConditionUnknown {
		message = "current state of the gateway is unknown"
		reason = gatewayapiv1.GatewayReasonPending
	}

	if err != nil {
		message += " error: " + err.Error()
	}

	cond := metav1.Condition{
		Type:               string(gatewayapiv1.GatewayConditionProgrammed),
		Status:             programmedStatus,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: generation,
	}
	return cond
}

func buildAcceptedCondition(generation int64, acceptedStatus metav1.ConditionStatus) metav1.Condition {
	cond := metav1.Condition{
		Type:               string(gatewayapiv1.GatewayConditionAccepted),
		Status:             acceptedStatus,
		Reason:             string(gatewayapiv1.GatewayReasonAccepted),
		Message:            fmt.Sprintf("Handled by %s", ControllerName),
		ObservedGeneration: generation,
	}
	return cond
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager, ctx context.Context) error {
	log := crlog.FromContext(ctx)
	clusterEventMapper := NewClusterEventMapper(log, mgr.GetClient())
	//TODO need to trigger gateway reconcile when gatewayclass params changes
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayapiv1.Gateway{}).
		Watches(&workv1.ManifestWork{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			log.V(3).Info("enqueuing gateways based on manifest work change ", "work namespace", o.GetNamespace())
			requests := []reconcile.Request{}
			annotations := o.GetAnnotations()
			if annotations == nil {
				log.V(3).Info("no parent or annotations on manifest work ", "work ns", o.GetNamespace(), "name", o.GetName())
				return requests
			}
			key := annotations["kuadrant.io/parent"]
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				log.Error(err, "failed to parse namespace and name from manifest work")
				return requests
			}
			log.Info("requeuing gateway ", "namespace", ns, "name", name)
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
			})
			return requests
		}), builder.OnlyMetadata).
		Watches(&clusterv1beta2.PlacementDecision{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			// kinda want to get the old and new object here and only queue if the clusters have changed
			// queue up gateways in this namespace
			log.V(3).Info("enqueuing gateways based on placementdecision change ", " namespace", o.GetNamespace())
			req := []reconcile.Request{}
			l := &gatewayapiv1.GatewayList{}
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
		Watches(&corev1.Secret{}, &ClusterEventHandler{client: r.Client}).
		Watches(
			&clusterv1.ManagedCluster{},
			handler.EnqueueRequestsFromMapFunc(clusterEventMapper.MapToGateway),
		).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			gateway, ok := object.(*gatewayapiv1.Gateway)
			if ok {
				shouldReconcile := slice.ContainsString(getSupportedClasses(), string(gateway.Spec.GatewayClassName))
				log.V(3).Info(" should reconcile", "gateway", gateway.Name, "with class ", gateway.Spec.GatewayClassName, "should ", shouldReconcile)
				return slice.ContainsString(getSupportedClasses(), string(gateway.Spec.GatewayClassName))
			}
			return true
		})).
		Complete(r)
}

//ToDo These need to be exposed by the kuadrant operator DNSPolicy APIs

const (
	ClustersLabelPrefix                                      = "clusters." + LabelPrefix
	MultiClusterIPAddressType       gatewayapiv1.AddressType = LabelPrefix + "MultiClusterIPAddress"
	MultiClusterHostnameAddressType gatewayapiv1.AddressType = LabelPrefix + "MultiClusterHostnameAddress"
)

// AddressTypeToMultiCluster returns a multi cluster version of the address type
// and a bool to indicate that provided address type was converted. If not - original type is returned
func AddressTypeToMultiCluster(address gatewayapiv1.GatewayAddress) (gatewayapiv1.AddressType, bool) {
	if *address.Type == gatewayapiv1.IPAddressType {
		return MultiClusterIPAddressType, true
	} else if *address.Type == gatewayapiv1.HostnameAddressType {
		return MultiClusterHostnameAddressType, true
	}
	return *address.Type, false
}

// AddressTypeToSingleCluster converts provided multicluster address to single cluster version
// the bool indicates a successful conversion
func AddressTypeToSingleCluster(address gatewayapiv1.GatewayAddress) (gatewayapiv1.AddressType, bool) {
	if *address.Type == MultiClusterIPAddressType {
		return gatewayapiv1.IPAddressType, true
	} else if *address.Type == MultiClusterHostnameAddressType {
		return gatewayapiv1.HostnameAddressType, true
	}
	return *address.Type, false
}
