package testutil

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"

	"github.com/goombaio/namegenerator"
	ocm_cluster_v1 "open-cluster-management.io/api/cluster/v1"
	ocm_cluster_v1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	ocm_cluster_v1beta2 "open-cluster-management.io/api/cluster/v1beta2"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	mgcv1alpha1 "github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

const (
	GatewayClassName      = "kuadrant-multi-cluster-gateway-instance-per-cluster"
	ManagedClusterSetName = "gateway-clusters"
	PlacementLabel        = "cluster.open-cluster-management.io/placement"
	ClusterSetLabelKey    = "test-ingress-cluster"
	ClusterSetLabelValue  = "true"

	// configuration environment variables
	managedZoneEnvvar            = "TEST_MANAGED_ZONE"
	hubNamespaceEnvvar           = "TEST_HUB_NAMESPACE"
	hubKubeContextEnvvar         = "TEST_HUB_KUBE_CONTEXT"
	spokeKubeContextPrefixEnvvar = "TEST_SPOKE_KUBE_CONTEXT_PREFIX"
	spokeClusterCountEnvvar      = "MGC_WORKLOAD_CLUSTERS_COUNT"
	ocmSingleEnvvar              = "OCM_SINGLE"
)

type SuiteConfig struct {
	cpClient     client.Client
	dpClients    []client.Client
	hubNamespace string
	managedZone  string
	cleanupList  []client.Object
}

func (cfg *SuiteConfig) Build() error {

	// Load test suite configuration from the environment
	if cfg.hubNamespace = os.Getenv(hubNamespaceEnvvar); cfg.hubNamespace == "" {
		return fmt.Errorf("env variable '%s' must be set", hubNamespaceEnvvar)
	}
	if cfg.managedZone = os.Getenv(managedZoneEnvvar); cfg.managedZone == "" {
		return fmt.Errorf("env variable '%s' must be set", managedZoneEnvvar)
	}

	var hubKubeContext string
	if hubKubeContext = os.Getenv(hubKubeContextEnvvar); hubKubeContext == "" {
		return fmt.Errorf("env variable '%s' must be set", hubKubeContextEnvvar)
	}

	var spokeClustersCount int
	var spokeKubeContextPrefix string
	if count := os.Getenv(spokeClusterCountEnvvar); count == "" {
		spokeClustersCount = 0
		if os.Getenv(hubNamespaceEnvvar) == "" {
			return fmt.Errorf("%s must be set if %s is 0", ocmSingleEnvvar, spokeClusterCountEnvvar)
		}
	} else {
		var err error
		if spokeClustersCount, err = strconv.Atoi(count); err != nil {
			return fmt.Errorf("'%s' should be a number", spokeClusterCountEnvvar)
		}
	}

	if spokeClustersCount != 0 {
		if spokeKubeContextPrefix = os.Getenv(spokeKubeContextPrefixEnvvar); spokeKubeContextPrefix == "" {
			return fmt.Errorf("'%s' should be set if '%s' is greater than zero", spokeKubeContextPrefixEnvvar, spokeClusterCountEnvvar)
		}
	}

	// Create controlplane cluster client
	restcfg, err := loadKubeconfig(hubKubeContext)
	if err != nil {
		return err
	}
	err = gatewayapi.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = ocm_cluster_v1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = ocm_cluster_v1beta2.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = ocm_cluster_v1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = mgcv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}

	cfg.cpClient, err = client.New(restcfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Fatal(err)
		return err
	}

	// Create spoke cluster clients
	if spokeClustersCount > 0 {
		cfg.dpClients = make([]client.Client, spokeClustersCount)
		for i := 0; i < spokeClustersCount; i++ {
			restcfg, err := loadKubeconfig(fmt.Sprintf("%s-%d", spokeKubeContextPrefix, i+1))
			if err != nil {
				return err
			}
			cfg.dpClients[i], err = client.New(restcfg, client.Options{Scheme: scheme.Scheme})
			if err != nil {
				return err
			}
		}
	} else {
		// use the hub cluster as spoke if no standalone spoke
		// clusters have been configured
		cfg.dpClients = []client.Client{cfg.cpClient}
	}

	return nil
}

func (cfg *SuiteConfig) HubClient() client.Client { return cfg.cpClient }

func (cfg *SuiteConfig) SpokeClient(idx int) client.Client { return cfg.dpClients[idx] }

func (cfg *SuiteConfig) GenerateName() string {
	nBig, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return namegenerator.NewNameGenerator(nBig.Int64()).Generate()
}

func (cfg *SuiteConfig) ManagedZone() string { return cfg.managedZone }

func (cfg *SuiteConfig) HubNamespace() string { return cfg.hubNamespace }

func (cfg *SuiteConfig) SpokeNamespace() string { return "kuadrant-" + cfg.hubNamespace }

func (cfg *SuiteConfig) ApplicationNamespace() string { return "kuadrant-" + cfg.hubNamespace }

func (cfg *SuiteConfig) ManagedClusterSet() string { return ManagedClusterSetName }

func loadKubeconfig(context string) (*rest.Config, error) {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: context},
	).ClientConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *SuiteConfig) InstallPrerequisites(ctx context.Context) error {
	cfg.cleanupList = []client.Object{}

	// label the managedclusters (at the moment we just label them all)
	// NOTE: this action won't be reverted after the suite finishes
	clusterList := &ocm_cluster_v1.ManagedClusterList{}
	if err := cfg.HubClient().List(ctx, clusterList); err != nil {
		return err
	}
	if len(clusterList.Items) == 0 {
		return fmt.Errorf("no managedclusters found in the Hub")
	}
	for _, cluster := range clusterList.Items {
		patch := client.MergeFrom(cluster.DeepCopy())
		if cluster.ObjectMeta.Labels != nil {
			cluster.ObjectMeta.Labels[ClusterSetLabelKey] = ClusterSetLabelValue
		} else {
			cluster.ObjectMeta.Labels = map[string]string{ClusterSetLabelKey: ClusterSetLabelValue}
		}
		if err := cfg.HubClient().Patch(ctx, &cluster, patch); err != nil {
			return err
		}
	}

	// ensure Namespace
	{
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cfg.HubNamespace()}}
		created, err := cfg.Ensure(ctx, namespace)
		if err != nil {
			return err
		}

		if created {
			cfg.cleanupList = append(cfg.cleanupList, namespace)
		}
	}

	// TODO ensure ManagedZone: right now this is added by local-setup

	// ensure ManagedClusterSet
	{
		managedclusterset := &ocm_cluster_v1beta2.ManagedClusterSet{
			ObjectMeta: metav1.ObjectMeta{Name: ManagedClusterSetName},
			Spec: ocm_cluster_v1beta2.ManagedClusterSetSpec{
				ClusterSelector: ocm_cluster_v1beta2.ManagedClusterSelector{
					SelectorType: ocm_cluster_v1beta2.LabelSelector,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							ClusterSetLabelKey: ClusterSetLabelValue,
						},
					},
				},
			},
		}
		created, err := cfg.Ensure(ctx, managedclusterset)
		if err != nil {
			return err
		}

		if created {
			cfg.cleanupList = append(cfg.cleanupList, managedclusterset)
		}
	}

	// ensure ManagedClusterSetBinding
	{
		managedclustersetbinding := &ocm_cluster_v1beta2.ManagedClusterSetBinding{
			ObjectMeta: metav1.ObjectMeta{Name: ManagedClusterSetName, Namespace: cfg.HubNamespace()},
			Spec: ocm_cluster_v1beta2.ManagedClusterSetBindingSpec{
				ClusterSet: ManagedClusterSetName,
			},
		}
		created, err := cfg.Ensure(ctx, managedclustersetbinding)
		if err != nil {
			return err
		}

		if created {
			cfg.cleanupList = append(cfg.cleanupList, managedclustersetbinding)
		}
	}

	// ensure GatewayClass
	{
		gatewayclass := &gatewayapi.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{Name: GatewayClassName},
			Spec:       gatewayapi.GatewayClassSpec{ControllerName: "kuadrant.io/mgc-gw-controller"},
		}

		created, err := cfg.Ensure(ctx, gatewayclass)
		if err != nil {
			return err
		}

		if created {
			cfg.cleanupList = append(cfg.cleanupList, gatewayclass)
		}
	}

	return nil
}

func (cfg *SuiteConfig) Cleanup(ctx context.Context) error {

	for _, o := range cfg.cleanupList {
		// Don't check for errors so all objects are deleted. Errors can be returned if for example the
		// namespace is deleted first, but we don't care
		_ = cfg.HubClient().Delete(ctx, o, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}

	return nil
}

func (cfg *SuiteConfig) Exists(ctx context.Context, o client.Object) (bool, error) {

	if err := cfg.HubClient().Get(ctx, client.ObjectKeyFromObject(o), o); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func (cfg *SuiteConfig) Ensure(ctx context.Context, o client.Object) (bool, error) {

	exists, err := cfg.Exists(ctx, o)
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	if err := cfg.HubClient().Create(ctx, o); err != nil && !apierrors.IsAlreadyExists(err) {
		return false, err
	}

	return true, err
}
