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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mgcv1alpha1 "github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"
	ocmclusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	ocmclusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"

	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	GatewayClassName      = "kuadrant-multi-cluster-gateway-instance-per-cluster"
	ManagedClusterSetName = "gateway-clusters"
	PlacementLabel        = "cluster.open-cluster-management.io/placement"
	ClusterSetLabelKey    = "test-ingress-cluster"
	ClusterSetLabelValue  = "true"

	// configuration environment variables
	managedZoneEnvVar            = "TEST_MANAGED_ZONE"
	hubNamespaceEnvVar           = "TEST_HUB_NAMESPACE"
	hubKubeContextEnvVar         = "TEST_HUB_KUBE_CONTEXT"
	spokeKubeContextPrefixEnvVar = "TEST_SPOKE_KUBE_CONTEXT_PREFIX"
	spokeClusterCountEnvVar      = "MGC_WORKLOAD_CLUSTERS_COUNT"
	ocmSingleEnvVar              = "OCM_SINGLE"
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
	if cfg.hubNamespace = os.Getenv(hubNamespaceEnvVar); cfg.hubNamespace == "" {
		return fmt.Errorf("env variable '%s' must be set", hubNamespaceEnvVar)
	}
	if cfg.managedZone = os.Getenv(managedZoneEnvVar); cfg.managedZone == "" {
		return fmt.Errorf("env variable '%s' must be set", managedZoneEnvVar)
	}

	var hubKubeContext string
	if hubKubeContext = os.Getenv(hubKubeContextEnvVar); hubKubeContext == "" {
		return fmt.Errorf("env variable '%s' must be set", hubKubeContextEnvVar)
	}

	var spokeClustersCount int
	var spokeKubeContextPrefix string
	if count := os.Getenv(spokeClusterCountEnvVar); count == "" {
		spokeClustersCount = 0
		if os.Getenv(hubNamespaceEnvVar) == "" {
			return fmt.Errorf("%s must be set if %s is 0", ocmSingleEnvVar, spokeClusterCountEnvVar)
		}
	} else {
		var err error
		if spokeClustersCount, err = strconv.Atoi(count); err != nil {
			return fmt.Errorf("'%s' should be a number", spokeClusterCountEnvVar)
		}
	}

	if spokeClustersCount != 0 {
		if spokeKubeContextPrefix = os.Getenv(spokeKubeContextPrefixEnvVar); spokeKubeContextPrefix == "" {
			return fmt.Errorf("'%s' should be set if '%s' is greater than zero", spokeKubeContextPrefixEnvVar, spokeClusterCountEnvVar)
		}
	}

	// Create controlplane cluster client
	restCfg, err := loadKubeConfig(hubKubeContext)
	if err != nil {
		return err
	}
	err = gatewayapi.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = ocmclusterv1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = ocmclusterv1beta2.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = ocmclusterv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	err = mgcv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}

	cfg.cpClient, err = client.New(restCfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Fatal(err)
		return err
	}

	// Create spoke cluster clients
	if spokeClustersCount > 0 {
		cfg.dpClients = make([]client.Client, spokeClustersCount)
		for i := 0; i < spokeClustersCount; i++ {
			restCfg, err = loadKubeConfig(fmt.Sprintf("%s-%d", spokeKubeContextPrefix, i+1))
			if err != nil {
				return err
			}
			cfg.dpClients[i], err = client.New(restCfg, client.Options{Scheme: scheme.Scheme})
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

func loadKubeConfig(context string) (*rest.Config, error) {
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
	clusterList := &ocmclusterv1.ManagedClusterList{}
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
		managedClusterSet := &ocmclusterv1beta2.ManagedClusterSet{
			ObjectMeta: metav1.ObjectMeta{Name: ManagedClusterSetName},
			Spec: ocmclusterv1beta2.ManagedClusterSetSpec{
				ClusterSelector: ocmclusterv1beta2.ManagedClusterSelector{
					SelectorType: ocmclusterv1beta2.LabelSelector,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							ClusterSetLabelKey: ClusterSetLabelValue,
						},
					},
				},
			},
		}
		created, err := cfg.Ensure(ctx, managedClusterSet)
		if err != nil {
			return err
		}

		if created {
			cfg.cleanupList = append(cfg.cleanupList, managedClusterSet)
		}
	}

	// ensure ManagedClusterSetBinding
	{
		managedClusterSetBinding := &ocmclusterv1beta2.ManagedClusterSetBinding{
			ObjectMeta: metav1.ObjectMeta{Name: ManagedClusterSetName, Namespace: cfg.HubNamespace()},
			Spec: ocmclusterv1beta2.ManagedClusterSetBindingSpec{
				ClusterSet: ManagedClusterSetName,
			},
		}
		created, err := cfg.Ensure(ctx, managedClusterSetBinding)
		if err != nil {
			return err
		}

		if created {
			cfg.cleanupList = append(cfg.cleanupList, managedClusterSetBinding)
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
		if k8serrors.IsNotFound(err) {
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

	if err := cfg.HubClient().Create(ctx, o); err != nil && !k8serrors.IsAlreadyExists(err) {
		return false, err
	}

	return true, err
}
