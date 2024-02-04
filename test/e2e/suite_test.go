//go:build e2e

package e2e

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kuadrantvdns1alpha1 "github.com/kuadrant/kuadrant-dns-operator/api/v1alpha1"
	kuadrantv1alpha1 "github.com/kuadrant/kuadrant-operator/api/v1alpha1"

	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var (
	tconfig SuiteConfig
	// testSuiteID is a randomly generated identifier for the test suite
	testSuiteID string
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Tests Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	tconfig = SuiteConfig{}
	err := tconfig.Build()
	Expect(err).NotTo(HaveOccurred())

	err = kuadrantv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = kuadrantvdns1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = tconfig.InstallPrerequisites(ctx)
	Expect(err).NotTo(HaveOccurred())

	testSuiteID = "t-e2e-" + tconfig.GenerateName()
})

var _ = AfterSuite(func(ctx SpecContext) {
	err := tconfig.Cleanup(ctx)
	Expect(err).ToNot(HaveOccurred())
})

func ResolverForDomainName(domainName string) *net.Resolver {
	nameservers, err := net.LookupNS(domainName)
	Expect(err).ToNot(HaveOccurred())
	GinkgoWriter.Printf("[debug] authoritative nameserver used for DNS record resolution: %s\n", nameservers[0].Host)

	authoritativeResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, network, strings.Join([]string{nameservers[0].Host, "53"}, ":"))
		},
	}
	return authoritativeResolver
}
