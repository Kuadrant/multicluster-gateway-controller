package placement

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/conditions"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func init() {
	utilruntime.Must(workv1.AddToScheme(scheme.Scheme))
}

const (
	OCMPlacementLabel = "cluster.open-cluster-management.io/placement"
	rbacName          = "open-cluster-management:klusterlet-work:gateway"
	rbacManifest      = "gateway-rbac"
)

type ocmPlacer struct {
	workClient      workclient.Interface
	placementClient clusterclient.Interface
}

func NewOCMPlacer(restConfig *rest.Config) (*ocmPlacer, error) {
	wc, err := workclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	cc, err := clusterclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &ocmPlacer{
		placementClient: cc,
		workClient:      wc,
	}, nil
}

func (op *ocmPlacer) GetAddresses(ctx context.Context, gateway *gatewayv1beta1.Gateway, downstream string) ([]gatewayv1beta1.GatewayAddress, error) {
	workname := workName(gateway)
	addresses := []gatewayv1beta1.GatewayAddress{}
	rootMeta, _ := k8smeta.Accessor(gateway)
	mw, err := op.workClient.WorkV1().ManifestWorks(downstream).Get(ctx, workname, metav1.GetOptions{})
	if err != nil {
		return addresses, err
	}
	for _, m := range mw.Status.ResourceStatus.Manifests {
		if m.ResourceMeta.Group == gateway.GetObjectKind().GroupVersionKind().Group && m.ResourceMeta.Name == rootMeta.GetName() {
			for _, value := range m.StatusFeedbacks.Values {
				if value.Name == "addresses" {
					t := gatewayv1beta1.IPAddressType
					addresses = append(addresses, gatewayv1beta1.GatewayAddress{
						Type:  &t,
						Value: *value.Value.String,
					})
				}
			}
		}
	}
	return addresses, nil
}

func (op *ocmPlacer) ListenerTotalAttachedRoutes(ctx context.Context, gateway *gatewayv1beta1.Gateway, listenerName string, downstream string) (int, error) {
	workname := workName(gateway)
	rootMeta, _ := k8smeta.Accessor(gateway)
	mw, err := op.workClient.WorkV1().ManifestWorks(downstream).Get(ctx, workname, metav1.GetOptions{})
	if err != nil {
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

func workName(rootObj runtime.Object) string {
	kind := rootObj.GetObjectKind().GroupVersionKind().Kind
	rootMeta, _ := k8smeta.Accessor(rootObj)
	return strings.ToLower(fmt.Sprintf("%s-%s-%s", kind, rootMeta.GetNamespace(), rootMeta.GetName()))
}

// TODO (we will need the gateway controller to be triggered by a placement change)
// find the placement decision. create/update the manifestwork
// return the clusters it knows the gateway is placed on
func (op *ocmPlacer) Place(ctx context.Context, upStreamGateway *gatewayv1beta1.Gateway, downStreamGateway *gatewayv1beta1.Gateway, children ...metav1.Object) (sets.Set[string], error) {
	//PoC currently each object is put into its own manifestwork. This shouldn't be needed but would require finding the manifest work and replacing the existing object
	log := log.Log
	log.V(3).Info("placement: placing ", "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
	workname := workName(upStreamGateway)
	emyptySet := sets.Set[string](sets.NewString())
	// where the placement decision says to place the gateway
	placementTargets, err := op.GetClusters(ctx, upStreamGateway)
	if err != nil {
		return emyptySet, err
	}
	log.V(3).Info("placement: ", "targets", placementTargets.UnsortedList(), "gateway", downStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
	existingClusters, err := op.GetPlacedClusters(ctx, upStreamGateway)
	if err != nil {
		return emyptySet, err
	}

	// not in target clusters so need to be removed
	removeFrom := existingClusters.Difference(placementTargets)
	log.V(3).Info("placement: ", "removeFrom", removeFrom.UnsortedList(), "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
	// if being deleted entirely remove manifest from all existing clusters
	if upStreamGateway.GetDeletionTimestamp() != nil {
		log.V(3).Info("placement: ", "deleting gateway from ", existingClusters.UnsortedList(), "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		for _, cluster := range existingClusters.UnsortedList() {
			// being deleted need to remove from clusters
			if err := op.workClient.WorkV1().ManifestWorks(cluster).Delete(ctx, workname, metav1.DeleteOptions{}); err != nil {
				// use a multi-error
				return existingClusters, err
			}
			existingClusters.Delete(cluster)
		}
		return existingClusters, nil
	}
	objects := []metav1.Object{downStreamGateway}
	objects = append(objects, children...)
	for _, cluster := range placementTargets.UnsortedList() {
		log.V(3).Info("placement: ", "adding gateway rbac to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		if err := op.defaultRBAC(ctx, cluster); err != nil {
			log.V(3).Info("placement: ", "adding gateway rbac to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace, "error", err)
			return existingClusters, err
		}
		log.V(3).Info("placement: ", "adding gateway to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		if err := op.createUpdateClusterManifests(ctx, workname, upStreamGateway, downStreamGateway, cluster, objects...); err != nil {
			log.V(3).Info("placement: ", "adding gateway to cluster ", cluster, "gateway", upStreamGateway.Name, "error", err)
			return existingClusters, err
		}
		log.V(3).Info("placement: ", "added gateway to cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		existingClusters.Insert(cluster)
	}

	// remove from remove
	for _, cluster := range removeFrom.UnsortedList() {
		log.V(3).Info("placement: ", "removing gateway from cluster ", cluster, "gateway", upStreamGateway.Name, "gateway ns", upStreamGateway.Namespace)
		//todo remove rbac
		if err := op.workClient.WorkV1().ManifestWorks(cluster).Delete(ctx, workname, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			// use a multi-error
			return existingClusters, err
		}
		if err := op.workClient.WorkV1().ManifestWorks(cluster).Delete(ctx, rbacManifest, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			// use a multi-error
			return existingClusters, err
		}
		existingClusters.Delete(cluster)
	}

	return existingClusters, nil
}

func (op *ocmPlacer) GetPlacedClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error) {
	selector := fmt.Sprintf("kuadrant.io/manifestKey=%s", workName(gateway))
	existing, err := op.workClient.WorkV1().ManifestWorks("").List(ctx, metav1.ListOptions{LabelSelector: selector})
	existingClusters := sets.Set[string](sets.NewString())
	if err != nil {
		return existingClusters, err
	}
	//where the gateway currently exists

	for _, e := range existing.Items {
		applied := conditions.GetConditionByType(e.Status.Conditions, string(workv1.ManifestApplied))
		if applied != nil && applied.Status == metav1.ConditionTrue {
			existingClusters = existingClusters.Insert(e.GetNamespace())
		}
	}
	return existingClusters, nil
}

func (op *ocmPlacer) GetClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error) {
	rootMeta, _ := k8smeta.Accessor(gateway)
	labels := rootMeta.GetLabels()
	selectedPlacement := labels[OCMPlacementLabel]
	targetClusters := sets.Set[string](sets.NewString())
	if selectedPlacement == "" {
		return targetClusters, nil
	}

	// find the placement decsion
	pdList, err := op.placementClient.ClusterV1beta1().PlacementDecisions(rootMeta.GetNamespace()).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{OCMPlacementLabel: selectedPlacement}})})
	if err != nil {
		return targetClusters, err
	}
	for _, pd := range pdList.Items {
		for _, d := range pd.Status.Decisions {
			targetClusters.Insert(d.ClusterName)
		}
	}
	return targetClusters, nil
}

func (op *ocmPlacer) createUpdateClusterManifests(ctx context.Context, manifestName string, upstream metav1.Object, downstream metav1.Object, cluster string, obj ...metav1.Object) error {
	log := log.Log
	// set up gateway manifest
	key, err := cache.MetaNamespaceKeyFunc(upstream)
	if err != nil {
		return err
	}
	work := workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifestName,
			Namespace: cluster,
			Labels:    map[string]string{"kuadrant.io": "managed", "kuadrant.io/manifestKey": manifestName},
			// this is crap, there has to be a better way to map to the parent object perhaps using cache
			// there is also a resource https://github.com/open-cluster-management-io/api/blob/main/work/v1alpha1/types_manifestworkreplicaset.go that we may migrate to which would solve this
			Annotations: map[string]string{"kuadrant.io/parent": key},
		},
	}
	objManifests, err := op.manifest(obj...)
	if err != nil {
		return err
	}
	log.V(3).Info("placement:", "manifests prepared", len(objManifests))

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
			FeedbackRules: []workv1.FeedbackRule{
				{
					JsonPaths: []workv1.JsonPath{
						{
							Name: "addresses",
							Path: ".status.addresses[?(@.type==\"IPAddress\")].value",
						},
						{
							Name: "listenerTestAttachedRoutes",
							Path: ".status.listeners[?(@.name==\"test\")].attachedRoutes",
						},
						{
							Name: "listenerAPIAttachedRoutes",
							Path: ".status.listeners[?(@.name==\"api\")].attachedRoutes",
						},
					},
					Type: workv1.JSONPathsType,
				},
			},
		},
	}
	log.V(3).Info("placement: creating updating maniftests for ", "cluster", cluster)
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
	mw, err := op.workClient.WorkV1().ManifestWorks(cluster).Get(ctx, m.Name, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		log.Log.V(3).Info("placement: manifest not found creating it ")
		if _, err := op.workClient.WorkV1().ManifestWorks(cluster).Create(ctx, &m, metav1.CreateOptions{}); err != nil {
			log.Log.V(3).Info("placement:  creating manifest ", "error", err)
			return err
		}
		return nil
	}
	if !equality.Semantic.DeepEqual(mw.Spec, m.Spec) {
		log.Log.V(3).Info("placment: manifest found updating it ")
		mw.Spec = m.Spec
		if _, err := op.workClient.WorkV1().ManifestWorks(cluster).Update(ctx, mw, metav1.UpdateOptions{}); err != nil {
			log.Log.V(3).Info("placement:  updating manifest ", "error", err)
			return err
		}
	}

	return nil

}
