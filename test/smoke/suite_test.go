package smoke

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	testutil "github.com/Kuadrant/multi-cluster-traffic-controller/test/util"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var tconfig testutil.SuiteConfig

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Smoke Tests Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	tconfig = testutil.SuiteConfig{}
	err := tconfig.Build()
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterSuite(func() {
	// By("tearing down the test environment")
	// err := testEnv.Stop()
	// Expect(err).NotTo(HaveOccurred())
})
