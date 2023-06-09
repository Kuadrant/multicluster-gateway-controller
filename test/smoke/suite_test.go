package smoke

import (
	"testing"

	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var tconfig SuiteConfig

func TestAPIs(t *testing.T) {
	ConditionalTestSkip(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Smoke Tests Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	tconfig = SuiteConfig{}
	err := tconfig.Build()
	Expect(err).NotTo(HaveOccurred())

	err = tconfig.InstallPrerequisites(ctx)
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterSuite(func(ctx SpecContext) {
	tconfig.Cleanup(ctx)
})
