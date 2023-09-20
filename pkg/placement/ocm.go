package placement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	clusterv1 "open-cluster-management.io/api/cluster/v1"
	placement "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"

	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	k8smeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/gracePeriod"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	OCMPlacementLabel          = "cluster.open-cluster-management.io/placement"
	rbacName                   = "open-cluster-management:klusterlet-work:gateway"
	rbacManifest               = "gateway-rbac"
	WorkManifestLabel          = "kuadrant.io/manifestKey"
	PlacementDecisionFinalizer = "kuadrant.io/finalizer"
)

var logger = log.Log

type ocmPlacer struct {
	c client.Client
}

func NewOCMPlacer(c client.Client) *ocmPlacer {

	return &ocmPlacer{
		c: c,
	}
}

func (op *ocmPlacer) GetAddresses(ctx context.Context, gateway *gatewayv1beta1.Gateway, downstream string) ([]gatewayv1beta1.GatewayAddress, error) {
	workname := WorkName(gateway)
	addresses := []gatewayv1beta1.GatewayAddress{}
	rootMeta, _ := k8smeta.Accessor(gateway)
	mw := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workname,
			Namespace: downstream,
		},
	}
	var err error
	if err = op.c.Get(ctx, client.ObjectKeyFromObject(mw), mw, &client.GetOptions{}); err != nil {
		return addresses, err
	}
	for _, m := range mw.Status.ResourceStatus.Manifests {
		if m.ResourceMeta.Group == gateway.GetObjectKind().GroupVersionKind().Group && m.ResourceMeta.Name == rootMeta.GetName() {
			for _, value := range m.StatusFeedbacks.Values {
				if value.Name == "addresses" {
					err = json.Unmarshal([]byte(*value.Value.JsonRaw), &addresses)
					break
				}
			}
		}
	}
	return addresses, err
}

func (op *ocmPlacer) ListenerTotalAttachedRoutes(ctx context.Context, gateway *gatewayv1beta1.Gateway, listenerName string, downstream string) (int, error) {
	workname := WorkName(gateway)
	rootMeta, _ := k8smeta.Accessor(gateway)
	mw := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workname,
			Namespace: downstream,
		},
	}
	if err := op.c.Get(ctx, client.ObjectKeyFromObject(mw), mw, &client.GetOptions{}); err != nil {
		return 0, err
	}
	for _, m := range mw.Status.ResourceStatus.Manifests {
		if m.ResourceMeta.Group == gateway.GetObjectKind().GroupVersionKind().Group && m.ResourceMeta.Name == rootMeta.GetName() {
			for _, value := range m.StatusFeedbacks.Values {
				attachedRoutesStatusKey := strings.ToLower(fmt.Sprintf("listener%sAttachedRoutes", listenerName))
				if strings.ToLower(value.Name) == attachedRoutesStatusKey {
					return int(*value.Value.Integer), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("no listener %s status found", listenerName)

}

func WorkName(rootObj runtime.Object) string {
	kind := rootObj.GetObjectKind().GroupVersionKind().Kind
	rootMeta, _ := k8smeta.Accessor(rootObj)
	return strings.ToLower(fmt.Sprintf("%s-%s-%s", kind, rootMeta.GetNamespace(), rootMeta.GetName()))
}

func hasPlacementLabel(upstreamGateway *gatewayv1beta1.Gateway) bool {
	if upstreamGateway.Labels == nil {
		return false
	}
	_, ok := upstreamGateway.Labels[OCMPlacementLabel]
	return ok
}

// Place ensures the gateway is placed onto the chosen clusters by creating the required manifestwork resources
func (op *ocmPlacer) Place(ctx context.Context, upStreamGateway *gatewayv1beta1.Gateway, downStreamGateway *gatewayv1beta1.Gateway, children ...metav1.Object) (sets.Set[string], string, error) {

	// load existing placement decisions and build existing clusters
	// load targetted placement decisons and build expected clusters
	var (
		existingPlacementTarget = ""
		expectedPlacementTarget = ""
	)
	if existingPlacement, ok := upStreamGateway.Labels["kuadrant.io/placed"]; ok {
		existingPlacementTarget = existingPlacement
	}
	if expectedPlacement, ok := upStreamGateway.Labels[OCMPlacementLabel]; ok {
		expectedPlacementTarget = expectedPlacement
	}
	// assume no existing clusters or expected
	existingClusters := sets.Set[string](sets.NewString())
	expectedClusters := sets.Set[string](sets.NewString())

	existingPlacementDecisions, err := op.GetPlacementDecsions(ctx, existingPlacementTarget, upStreamGateway.Namespace)
	if err != nil {
		return existingClusters, expectedPlacementTarget, fmt.Errorf("failed to get existing placement decisons %w", err)
	}
	existingClusters = op.GetTargetClusters(existingPlacementDecisions)

	expectedPlacementDecisions, err := op.GetPlacementDecsions(ctx, expectedPlacementTarget, upStreamGateway.Namespace)
	if err != nil {
		return existingClusters, expectedPlacementTarget, fmt.Errorf("failed to get expected placement decisons %w", err)
	}
	expectedClusters = op.GetTargetClusters(expectedPlacementDecisions)

	workname := WorkName(upStreamGateway)
	// if no expected placement or upstream being deleted  remove from all existing clusters
	if expectedPlacementTarget == "" || upStreamGateway.DeletionTimestamp != nil {
		logger.V(3).Info("removing gateway from all existing gateways ", "placement target ", expectedPlacementTarget, "gatway deletion", upStreamGateway.DeletionTimestamp)
		var removeErr error
		for _, existingCluster := range existingClusters.UnsortedList() {
			if err := op.removeGatewayFromSpoke(ctx, existingCluster, workname); err != nil {
				removeErr = errors.Join(err)
				existingClusters.Delete(existingCluster)
			}
		}
		if removeErr != nil {
			return existingClusters, expectedPlacementTarget, removeErr
		}
		if err := op.removeFinalizerPlacementDecisons(ctx, existingPlacementDecisions); err != nil {
			return existingClusters, expectedPlacementTarget, err
		}
		return existingClusters, existingPlacementTarget, nil
	}
	if existingPlacementTarget == expectedPlacementTarget {
		// nothing to do return existing clusters
		logger.V(3).Info("no placement change needed ", "existing placement", existingPlacementTarget, "expectd placement", expectedPlacementTarget)
		return existingClusters, expectedPlacementTarget, nil
	}
	// the placement has changed or been applied
	logger.V(3).Info("placement: placing ", "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace, "expected placement", expectedPlacementTarget, "existing placement", existingPlacementTarget)

	logger.V(3).Info("placement: ", "targets", expectedClusters.UnsortedList(), "gateway", downStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)

	// build what will be placed into the down stream
	objects := []metav1.Object{downStreamGateway}
	objects = append(objects, children...)

	for _, cluster := range expectedClusters.UnsortedList() {
		logger.V(3).Info("placement: ", "adding gateway rbac to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		if err := op.defaultRBAC(ctx, cluster); err != nil {
			logger.V(3).Info("placement: ", "adding gateway rbac to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace, "error", err)
			return existingClusters, expectedPlacementTarget, err
		}
		logger.V(3).Info("placement: ", "adding gateway to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		if err := op.createUpdateClusterManifests(ctx, workname, upStreamGateway, downStreamGateway, cluster, objects...); err != nil {
			logger.V(3).Info("placement: ", "adding gateway to cluster ", cluster, "gateway", upStreamGateway.Name, "error", err)
			return existingClusters, expectedPlacementTarget, err
		}
		logger.V(3).Info("placement: ", "added gateway to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		existingClusters.Insert(cluster)
	}
	// not in target clusters so need to be removed
	removeFrom := existingClusters.Difference(expectedClusters)
	logger.V(3).Info("placement: ", "removeFrom", removeFrom.UnsortedList(), "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)

	// remove from remove
	for _, cluster := range removeFrom.UnsortedList() {
		logger.V(3).Info("placement: ", "removing gateway from cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		var removeError error
		if err := op.removeGatewayFromSpoke(ctx, cluster, workname); err != nil {
			removeError = errors.Join(err)
		}
		if removeError != nil {
			return existingClusters, expectedPlacementTarget, removeError
		}
	}

	//placement done add and remove any finalizers from the no longer targeted placement decisons
	if err := op.removeFinlizerPlacementDecisons(ctx, existingPlacementDecisions); err != nil {
		return existingClusters, expectedPlacementTarget, fmt.Errorf("failed to remove finalizers from placement decisons for no longer targeted placements %w", err)
	}
	if err := op.addFinalizePlacementDecisions(ctx, expectedPlacementDecisions); err != nil {
		return existingClusters, expectedPlacementTarget, fmt.Errorf("failed to add finalizers after placement complete to targeted placement decisons %w", err)
	}

	return existingClusters, expectedPlacementTarget, nil
}

var placementCondType = "Placement"
var placementReason = "ResolvedPlacementDecision"

func (op *ocmPlacer) removeGatewayFromSpoke(ctx context.Context, cluster string, workname string) error {
	w := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workname,
			Namespace: cluster,
		},
	}
	err := op.c.Get(ctx, client.ObjectKeyFromObject(w), w)
	if err != nil {
		return fmt.Errorf("failed to remove gateway from spoke %s : %w", cluster, err)
	}

	// Check if the ManagedCluster still exists,
	// otherwise delete without any grace period.
	// This can happen if a ManagedCluster is deleted,
	// as opposed to just a placement decision changing
	ignoreGrace := false
	managedCluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster,
		},
	}
	err = op.c.Get(ctx, client.ObjectKeyFromObject(managedCluster), managedCluster)
	if k8serrors.IsNotFound(err) {
		logger.V(3).Info(fmt.Sprintf("ManagedCluster not found '%s', ignoring grace period", cluster))
		ignoreGrace = true
	}
	if err := gracePeriod.GracefulDelete(ctx, op.c, w, ignoreGrace); err != nil {
		return fmt.Errorf("failed graceful delete when removing gateway manifestwork from spoke cluster %s : %w", cluster, err)
	}

	logger.V(3).Info("graceful delete of gateway manifestwork complete, deleting RBAC")
	rbac := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster,
			Name:      rbacManifest,
		},
	}
	if err := op.c.Delete(ctx, rbac, &client.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete rbac manifestwoork when removing gateway from spoke cluster %s : %w ", cluster, err)
	}
	return nil
}

func (op *ocmPlacer) removeFinlizerPlacementDecisons(ctx context.Context, pds placement.PlacementDecisionList) error {
	var finalizerErr error
	for _, pd := range pds.Items {
		if controllerutil.RemoveFinalizer(&pd, PlacementDecisionFinalizer) {
			if err := op.c.Update(ctx, &pd); err != nil {
				finalizerErr = errors.Join(err)
			}
		}
	}
	return finalizerErr
}

func (op *ocmPlacer) addFinalizePlacementDecisions(ctx context.Context, pds placement.PlacementDecisionList) error {
	var finalizerErr error
	for _, pd := range pds.Items {
		if pd.DeletionTimestamp == nil {
			if controllerutil.AddFinalizer(&pd, PlacementDecisionFinalizer) {
				if err := op.c.Update(ctx, &pd); err != nil {
					finalizerErr = errors.Join(err)
				}
			}
		}
	}
	return finalizerErr
}

func (op ocmPlacer) removeFinalizerPlacementDecisons(ctx context.Context, pds placement.PlacementDecisionList) error {
	var finalizerErr error
	for _, pd := range pds.Items {
		if controllerutil.RemoveFinalizer(&pd, PlacementDecisionFinalizer) {
			if err := op.c.Update(ctx, &pd); err != nil {
				finalizerErr = errors.Join(err)
			}
		}
	}
	return finalizerErr
}

func (op *ocmPlacer) GetPlacementDecsions(ctx context.Context, targetPlacement, targetNS string) (placement.PlacementDecisionList, error) {
	pdList := &placement.PlacementDecisionList{}
	if targetPlacement == "" {
		return *pdList, nil
	}
	labelSelector := client.MatchingLabels{
		OCMPlacementLabel: targetPlacement,
	}

	err := op.c.List(ctx, pdList, &client.ListOptions{Namespace: targetNS}, labelSelector)
	return *pdList, err
}

// GetPlacedClusters will return the list of clusters this gateway has been successfully placed on
func (op *ocmPlacer) GetPlacedClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error) {
	existing := &workv1.ManifestWorkList{}
	listOptions := client.MatchingLabels{
		WorkManifestLabel: WorkName(gateway),
	}
	existingClusters := sets.Set[string](sets.NewString())
	if err := op.c.List(ctx, existing, listOptions); err != nil {
		return existingClusters, err
	}
	//where the gateway currently exists

	for _, e := range existing.Items {
		deleting := e.DeletionTimestamp != nil
		applied := meta.IsStatusConditionTrue(e.Status.Conditions, string(workv1.ManifestApplied))
		if !deleting && applied {
			existingClusters = existingClusters.Insert(e.GetNamespace())
		}
	}
	return existingClusters, nil
}

func (op *ocmPlacer) GetTargetClusters(decisions placement.PlacementDecisionList) sets.Set[string] {
	targeted := sets.Set[string](sets.NewString())
	for _, pd := range decisions.Items {
		for _, c := range pd.Status.Decisions {
			targeted.Insert(c.ClusterName)
		}
	}
	return targeted
}

func (op *ocmPlacer) GetClusterGateway(ctx context.Context, gateway *gatewayv1beta1.Gateway, clusterName string) (dns.ClusterGateway, error) {
	var target dns.ClusterGateway
	workname := WorkName(gateway)
	mw := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workname,
			Namespace: clusterName,
		},
	}
	if err := op.c.Get(ctx, client.ObjectKeyFromObject(mw), mw, &client.GetOptions{}); err != nil {
		return target, err
	}

	mc := &clusterv1.ManagedCluster{}
	if err := op.c.Get(ctx, client.ObjectKey{Name: clusterName}, mc, &client.GetOptions{}); err != nil {
		return target, err
	}

	addresses, err := op.GetAddresses(ctx, gateway, clusterName)
	if err != nil {
		return target, err
	}
	target = *dns.NewClusterGateway(mc, addresses)
	return target, nil
}

func (op *ocmPlacer) createUpdateClusterManifests(ctx context.Context, manifestName string, upstream *gatewayv1beta1.Gateway, downstream *gatewayv1beta1.Gateway, cluster string, obj ...metav1.Object) error {
	// set up gateway manifest
	key, err := cache.MetaNamespaceKeyFunc(upstream)
	if err != nil {
		return err
	}
	work := workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifestName,
			Namespace: cluster,
			Labels:    map[string]string{"kuadrant.io": "managed", WorkManifestLabel: manifestName},
			// this is crap, there has to be a better way to map to the parent object perhaps using cache
			// there is also a resource https://github.com/open-cluster-management-io/api/blob/main/work/v1alpha1/types_manifestworkreplicaset.go that we may migrate to which would solve this
			Annotations: map[string]string{"kuadrant.io/parent": key},
		},
	}
	objManifests, err := op.manifest(obj...)
	if err != nil {
		return err
	}
	logger.V(3).Info("placement:", "manifests prepared", len(objManifests))

	work.Spec.Workload = workv1.ManifestsTemplate{
		Manifests: objManifests,
	}

	work.Spec.ManifestConfigs = []workv1.ManifestConfigOption{
		{
			ResourceIdentifier: workv1.ResourceIdentifier{
				Group:     "gateway.networking.k8s.io",
				Resource:  "gateways",
				Name:      downstream.GetName(),
				Namespace: downstream.GetNamespace(),
			},
		},
	}
	// using 0 index as there is only one config here
	work.Spec.ManifestConfigs[0].FeedbackRules = []workv1.FeedbackRule{
		{Type: workv1.JSONPathsType},
	}

	jsonPaths := []workv1.JsonPath{
		{
			Name: "addresses",
			Path: ".status.addresses",
		},
	}
	for _, l := range upstream.Spec.Listeners {
		jsonPaths = append(jsonPaths, workv1.JsonPath{
			Name: fmt.Sprintf("listener%sAttachedRoutes", l.Name),
			Path: fmt.Sprintf(".status.listeners[?(@.name==\"%s\")].attachedRoutes", l.Name),
		})
	}

	work.Spec.ManifestConfigs[0].FeedbackRules[0].JsonPaths = jsonPaths
	logger.V(3).Info("placement: creating updating maniftests for ", "cluster", cluster)
	return op.createUpdateManifest(ctx, cluster, work)

}

func (op *ocmPlacer) manifest(obj ...metav1.Object) ([]workv1.Manifest, error) {
	//TODO need to create an empty meta data to avoid problems with UID and resourceid
	manifests := []workv1.Manifest{}
	for _, o := range obj {
		meta, _ := k8smeta.Accessor(o)
		jsonData, err := json.Marshal(meta)
		if err != nil {
			return nil, err
		}

		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: jsonData}})
		//setup the ns TODO this is gross look for better options
		ns := v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: meta.GetNamespace(),
			},
		}
		jsonData, err = json.Marshal(ns)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: jsonData}})
	}
	return manifests, nil
}

func (op *ocmPlacer) defaultRBAC(ctx context.Context, clusterName string) error {
	var m = []workv1.Manifest{}
	cr := rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: rbacName,
		},
		Rules: []rbac.PolicyRule{
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"gateway.networking.k8s.io"},
				Resources: []string{"gateways"},
			},
		},
	}

	clusterRoleJSON, err := json.Marshal(cr)
	if err != nil {
		return err
	}

	crb := rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management:klusterlet-work:gateway",
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "open-cluster-management:klusterlet-work:gateway",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "klusterlet-work-sa",
				Namespace: "open-cluster-management-agent",
			},
		},
	}

	clusterRoleBindingJSON, err := json.Marshal(crb)
	if err != nil {
		return err
	}

	m = append(m, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: clusterRoleJSON}}, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: clusterRoleBindingJSON}})
	manifests := workv1.ManifestsTemplate{Manifests: []workv1.Manifest{}}
	manifests.Manifests = append(manifests.Manifests, m...)

	work := workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gateway-rbac",
			Namespace: clusterName,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: manifests,
		},
	}
	return op.createUpdateManifest(ctx, clusterName, work)
}

func (op *ocmPlacer) createUpdateManifest(ctx context.Context, cluster string, m workv1.ManifestWork) error {
	mw := &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
	}
	if err := op.c.Get(ctx, client.ObjectKeyFromObject(mw), mw, &client.GetOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.V(3).Info("placement: manifest not found creating it ", "cluster", mw.Namespace)
			if err := op.c.Create(ctx, &m, &client.CreateOptions{}); err != nil {
				return err
			}
			return nil
		}
	}

	if !equality.Semantic.DeepEqual(mw.Spec, m.Spec) {
		logger.V(3).Info("placement: manifest found updating it ")
		mw.Spec = m.Spec
		if err := op.c.Update(ctx, mw, &client.UpdateOptions{}); err != nil {
			logger.V(3).Info("placement:  updating manifest ", "error", err)
			return err
		}
	}

	return nil
}
