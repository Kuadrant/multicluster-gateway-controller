package testutil

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"

	"github.com/goombaio/namegenerator"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	GatewayClassName          = "mctc-gw-istio-external-instance-per-cluster"
	ClusterSelectorLabelKey   = "kuadrant.io/gateway-cluster-label-selector"
	ClusterSelectorLabelValue = "type=test"

	// configuration environment variables
	tenantNamespaceEnvvar        = "TEST_TENANT_NAMESPACE"
	managedZoneEnvvar            = "TEST_MANAGED_ZONE"
	controlplaneContextEnvvar    = "TEST_CONTROLPLANE_CONTEXT"
	dataplaneContextPrefixEnvvar = "TEST_DATAPLANE_CONTEXT_PREFIX"
	dataplaneClusterCountEnvvar  = "TEST_DATAPLANE_CLUSTER_COUNT"
)

type SuiteConfig struct {
	cpClient        client.Client
	dpClients       []client.Client
	nameGenerator   namegenerator.Generator
	tenantNamespace string
	managedZone     string
}

func (cfg *SuiteConfig) Build() error {

	// Load test suite configuration from the environment
	if cfg.tenantNamespace = os.Getenv(tenantNamespaceEnvvar); cfg.tenantNamespace == "" {
		return fmt.Errorf("env variable '%s' must be set", tenantNamespaceEnvvar)
	}
	if cfg.managedZone = os.Getenv(managedZoneEnvvar); cfg.managedZone == "" {
		return fmt.Errorf("env variable '%s' must be set", managedZoneEnvvar)
	}

	var dataplaneClustersCount int
	var dataplaneContextPrefix string
	if count := os.Getenv(dataplaneClusterCountEnvvar); count == "" {
		dataplaneClustersCount = 0
	} else {
		var err error
		if dataplaneClustersCount, err = strconv.Atoi(count); err != nil {
			return fmt.Errorf("'%s' should be a number", dataplaneClusterCountEnvvar)
		}
	}

	if dataplaneClustersCount != 0 {
		if dataplaneContextPrefix = os.Getenv(dataplaneContextPrefixEnvvar); dataplaneContextPrefix == "" {
			return fmt.Errorf("'%s' should be set if '%s' is greater than zero", dataplaneContextPrefixEnvvar, dataplaneClusterCountEnvvar)
		}
	}

	var controlplaneContext string
	if controlplaneContext = os.Getenv(controlplaneContextEnvvar); controlplaneContext == "" {
		return fmt.Errorf("env variable '%s' must be set", controlplaneContextEnvvar)
	}

	// Create controlplane cluster client
	restcfg, err := loadKubeconfig(controlplaneContext)
	if err != nil {
		return err
	}
	err = gatewayapi.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	cfg.cpClient, err = client.New(restcfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}

	// Create dataplane cluster clients
	if dataplaneClustersCount > 0 {
		cfg.dpClients = make([]client.Client, dataplaneClustersCount)
		for i := 0; i < dataplaneClustersCount; i++ {
			restcfg, err := loadKubeconfig(fmt.Sprintf("%s-%d", dataplaneContextPrefix, i+1))
			if err != nil {
				return err
			}
			cfg.dpClients[i], err = client.New(restcfg, client.Options{Scheme: scheme.Scheme})
			if err != nil {
				return err
			}
		}
	} else {
		cfg.dpClients = []client.Client{cfg.cpClient}
	}

	// Create a randomized name generator
	nBig, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	cfg.nameGenerator = namegenerator.NewNameGenerator(nBig.Int64())

	return nil
}

func (cfg *SuiteConfig) ControlPlaneClient() client.Client { return cfg.cpClient }

func (cfg *SuiteConfig) DataPlaneClient(idx int) client.Client { return cfg.dpClients[idx] }

func (cfg *SuiteConfig) GenerateName() string { return cfg.nameGenerator.Generate() }

func (cfg *SuiteConfig) ManagedZone() string { return cfg.managedZone }

func (cfg *SuiteConfig) TenantNamespace() string { return cfg.tenantNamespace }

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
