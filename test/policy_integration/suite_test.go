//go:build integration

/*
Copyright 2023 The MultiCluster Traffic Controller Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package policy_integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"
	ocmclusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	ocmworkv1 "open-cluster-management.io/api/work/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnshealthcheckprobe"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnspolicy"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/tlspolicy"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/health"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg             *rest.Config
	k8sClient       client.Client
	testEnv         *envtest.Environment
	ctx             context.Context
	cancel          context.CancelFunc
	logger          logr.Logger
	providerFactory = func(ctx context.Context, providerAccessor v1alpha2.ProviderAccessor) (dns.Provider, error) {
		return &dns.FakeProvider{
			Zones: []*dns.Zone{
				{
					ID:      testutil.Pointer(TestZoneID),
					DNSName: testutil.Pointer(TestZoneDNSName),
				},
			},
		}, nil
	}
)

func testClient() client.Client { return k8sClient }

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Policy Controller Suite")
}

var _ = BeforeSuite(func() {
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logger.WithName("suite_test")
	logf.SetLogger(logger)
	ctx, cancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../", "config", "policy-controller", "crd", "bases"),
			filepath.Join("../../", "config", "gateway-api", "crd", "standard"),
			filepath.Join("../../", "config", "cert-manager", "crd", "latest"),
			//needed for now TODO remove once no longer need managedcluster in dnspolicy
			filepath.Join("../../", "config", "ocm", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}
	// Disable the CustomResourceValidationExpressions feature gate.
	// Gateway API v1 CRDs include validation rules that, with this feature
	// enabled, are estimated too costly, and therefore, the CRD fails
	// to be registered. As a temporary solution, disable the feature.
	// TODO:  Find a way to increase the runtime cost budget so that the
	// feature can be re-enabled
	testEnv.ControlPlane.APIServer = &envtest.APIServer{}
	testEnv.ControlPlane.APIServer.Configure().Set("feature-gates", "CustomResourceValidationExpressions=false")

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = v1alpha2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = gatewayapiv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = certman.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ocmworkv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ocmclusterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ocmclusterv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		HealthProbeBindAddress: "0",
		Metrics:                metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).ToNot(HaveOccurred())

	healthQueue := health.NewRequestQueue(1 * time.Second)
	err = k8sManager.Add(healthQueue)
	Expect(err).ToNot(HaveOccurred())

	monitor := health.NewMonitor()
	err = k8sManager.Add(monitor)
	Expect(err).ToNot(HaveOccurred())

	healthServer := &testHealthServer{
		Port: 3333,
	}
	err = k8sManager.Add(healthServer)
	Expect(err).ToNot(HaveOccurred())

	dnsPolicyBaseReconciler := reconcilers.NewBaseReconciler(
		k8sManager.GetClient(), k8sManager.GetScheme(), k8sManager.GetAPIReader(),
		logger.WithName("dnspolicy"),
		k8sManager.GetEventRecorderFor("DNSPolicy"),
	)

	err = (&DNSPolicyReconciler{
		TargetRefReconciler: reconcilers.TargetRefReconciler{
			BaseReconciler: dnsPolicyBaseReconciler,
		},
		DNSProvider: providerFactory,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	tlsPolicyBaseReconciler := reconcilers.NewBaseReconciler(
		k8sManager.GetClient(), k8sManager.GetScheme(), k8sManager.GetAPIReader(),
		logger.WithName("tlspolicy"),
		k8sManager.GetEventRecorderFor("TLSPolicy"),
	)

	err = (&TLSPolicyReconciler{
		TargetRefReconciler: reconcilers.TargetRefReconciler{
			BaseReconciler: tlsPolicyBaseReconciler,
		},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&DNSHealthCheckProbeReconciler{
		Client:        k8sManager.GetClient(),
		HealthMonitor: monitor,
		Queue:         healthQueue,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
