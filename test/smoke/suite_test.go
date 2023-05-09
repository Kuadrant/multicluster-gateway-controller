package smoke

import (
	"testing"

	. "github.com/Kuadrant/multi-cluster-traffic-controller/test/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var tconfig SuiteConfig

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Smoke Tests Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	tconfig = SuiteConfig{}
	err := tconfig.Build()
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterSuite(func() {
	// By("tearing down the test environment")
	// err := testEnv.Stop()
	// Expect(err).NotTo(HaveOccurred())
})
