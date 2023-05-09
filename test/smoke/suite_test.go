package smoke

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/goombaio/namegenerator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var cpClient client.Client
var testEnv *envtest.Environment
var nameGenerator namegenerator.Generator

// TODO: move these 2 constants to env vars so
// the tests are not tied to a particular environment
const tenantNamespace = "mctc-tenant-unstable"
const managedZone = "hcg-stage.rhcloud.com"

const gwClassName = "mctc-gw-istio-external-instance-per-cluster"
const clusterSelectorLabelKey = "kuadrant.io/gateway-cluster-label-selector"
const clusterSelectorLabelValue = "type=test"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Smoke Tests Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var err error

	nBig, err := rand.Int(rand.Reader, big.NewInt(1000000))
	Expect(err).NotTo(HaveOccurred())
	nameGenerator = namegenerator.NewNameGenerator(nBig.Int64())

	// cfg is defined in this file globally.
	testEnv = &envtest.Environment{
		UseExistingCluster: pointer.BoolPtr(true),
	}
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = gatewayapi.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// control-plane client
	cpClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(cpClient).NotTo(BeNil())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
