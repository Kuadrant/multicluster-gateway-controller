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

package gateway_integration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
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
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/placement"
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
	providerFactory = func(ctx context.Context, managedZone *v1alpha1.ManagedZone) (dns.Provider, error) {
		return &dns.FakeProvider{}, nil
	}
)

func testClient() client.Client { return k8sClient }

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logger.WithName("suite_test")
	logf.SetLogger(logger)
	ctx, cancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../", "config", "crd", "bases"),
			filepath.Join("../../", "config", "gateway-api", "crd", "standard"),
			filepath.Join("../../", "config", "cert-manager", "crd", "latest"),
			filepath.Join("../../", "config", "ocm", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = gatewayv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// err = certman.AddToScheme(scheme.Scheme)
	// Expect(err).NotTo(HaveOccurred())

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
		MetricsBindAddress:     "0",
	})
	Expect(err).ToNot(HaveOccurred())

	plc := placement.NewOCMPlacer(k8sManager.GetClient())

	err = (&GatewayClassReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&GatewayReconciler{
		Client:    k8sManager.GetClient(),
		Scheme:    k8sManager.GetScheme(),
		Placement: plc,
	}).SetupWithManager(k8sManager, ctx)
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
