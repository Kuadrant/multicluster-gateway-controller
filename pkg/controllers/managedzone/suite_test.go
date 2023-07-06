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

package managedzone

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

const (
	EventuallyTimeoutMedium   = time.Second * 10
	ConsistentlyTimeoutMedium = time.Second * 60
	TestRetryIntervalMedium   = time.Millisecond * 250
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../../", "config", "crd", "bases"),
			filepath.Join("../../../", "config", "gateway-api", "crd", "standard"),
			filepath.Join("../../../", "config", "cert-manager", "crd", "v1.7.1"),
			filepath.Join("../../../", "config", "ocm", "crd"),
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

	err = certman.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = workv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = clusterv1beta2.AddToScheme(scheme.Scheme)
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

	dnsProvider := &dns.FakeProvider{}

	err = (&ManagedZoneReconciler{
		Client:      k8sManager.GetClient(),
		Scheme:      k8sManager.GetScheme(),
		DNSProvider: dnsProvider,
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

var _ = Describe("ManagedZoneReconciler", func() {
	Context("testing ManagedZone controller", func() {
		var managedZone *v1alpha1.ManagedZone

		BeforeEach(func() {
			managedZone = &v1alpha1.ManagedZone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example.com",
					Namespace: "default",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					ID:         "example.com",
					DomainName: "example.com",
				},
			}
		})

		AfterEach(func() {
			// Clean up managedZones
			mzList := &v1alpha1.ManagedZoneList{}
			err := k8sClient.List(ctx, mzList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, mz := range mzList.Items {
				err = k8sClient.Delete(ctx, &mz)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should accept a managed zone for this controller and allow deletion", func() {
			Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())

			createdMZ := &v1alpha1.ManagedZone{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: managedZone.Namespace, Name: managedZone.Name}, createdMZ)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, managedZone)).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: managedZone.Namespace, Name: managedZone.Name}, createdMZ)
				return errors.IsNotFound(err)
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})
	})
})
