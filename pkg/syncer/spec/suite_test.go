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

package spec

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	//+kubebuilder:scaffold:imports

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer/mutator"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg            *rest.Config
	k8sClient      client.Client
	testEnv        *envtest.Environment
	ctx            context.Context
	cancel         context.CancelFunc
	clusterID      = "test-cluster"
	GVRs           = []string{"gateways.v1beta1.gateway.networking.k8s.io", "secrets.v1"}
	controlPlaneNS = "mgc-tenant"
	dataPlaneNS    = "mgc-downstream"
)

const (
	TestTimeoutShort        = time.Second * 1
	TestTimeoutMedium       = time.Second * 10
	TestTimeoutLong         = time.Second * 30
	TestRetryIntervalMedium = time.Millisecond * 250
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	zapLogger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	log.SetLogger(zapLogger)
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../../", "config", "crd", "bases"),
			filepath.Join("../../../", "config", "gateway-api", "crd", "standard"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = gatewayv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	ctrl.SetLogger(zapLogger)
	dynamicClient, err := dynamic.NewForConfig(k8sManager.GetConfig())
	Expect(err).ToNot(HaveOccurred())

	//start syncers
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())
	specSyncConfig := syncer.Config{
		GVRs:               GVRs,
		InformerFactory:    informerFactory,
		ClusterID:          clusterID,
		NeverSyncedGVRs:    []string{},
		UpstreamNamespaces: []string{controlPlaneNS},
		DownstreamNS:       dataPlaneNS,
		Mutators: []syncer.Mutator{
			&mutator.JSONPatch{},
			&mutator.AnnotationCleaner{},
		},
	}

	SpecSyncer, err := NewSpecSyncer(clusterID, dynamicClient, dynamicClient, specSyncConfig)
	Expect(err).NotTo(HaveOccurred())

	go SpecSyncer.Start(ctx)

	specSyncRunnable := syncer.GetSyncerRunnable(specSyncConfig, syncer.InformerForAnnotatedGVR, SpecSyncer)

	log.Log.Info("adding syncer informer to manager")
	go func() {
		err := specSyncRunnable.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNS},
	}
	err = k8sClient.Create(ctx, ns)
	Expect(err).To(BeNil())

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

var _ = Describe("Spec Syncer", func() {
	Context("testing the spec syncer", func() {
		var gateway *gatewayv1beta1.Gateway
		var secret *v1.Secret
		hostname := gatewayv1beta1.Hostname("test.host")
		BeforeEach(func() {
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: controlPlaneNS,
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "test",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "listener-1",
							Hostname: &hostname,
							Port:     443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
						},
					},
				},
			}
			secret = &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: controlPlaneNS,
				},
			}
		})

		//ensure gateway and secret are removed after each test.
		AfterEach(func() {
			// Delete secret from control plane
			Expect(func() bool {
				err := k8sClient.Delete(ctx, secret)
				return err == nil || apierrors.IsNotFound(err)
			}()).To(BeTrue())

			// Confirm it's removed downstream
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: dataPlaneNS}, &v1.Secret{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Delete gateway from control plane
			Expect(func() bool {
				err := k8sClient.Delete(ctx, gateway)
				return err == nil || apierrors.IsNotFound(err)
			}()).To(BeTrue())

			// Confirm it's removed downstream
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: dataPlaneNS}, &gatewayv1beta1.Gateway{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should ignore an unannotated gateway and secret", func() {
			//create gateway and secret in control plane
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			createdGateway := &gatewayv1beta1.Gateway{}
			gatewayNamespacedName := types.NamespacedName{Name: gateway.Name, Namespace: controlPlaneNS}

			Expect(k8sClient.Create(ctx, secret)).To(BeNil())
			createdSecret := &v1.Secret{}
			secretNamespacedName := types.NamespacedName{Name: secret.Name, Namespace: controlPlaneNS}

			// Check it does exist in the control plane
			Expect(k8sClient.Get(ctx, gatewayNamespacedName, createdGateway)).To(BeNil())
			Expect(k8sClient.Get(ctx, secretNamespacedName, createdSecret)).To(BeNil())

			// Check it doesn't exist in the data plane
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Namespace: dataPlaneNS, Name: gatewayNamespacedName.Name}, createdGateway))).To(BeTrue())
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Namespace: dataPlaneNS, Name: secretNamespacedName.Name}, createdSecret))).To(BeTrue())
		})
		It("should delete downstream when deleted upstream", func() {
			metadata.AddAnnotation(gateway, syncer.MGC_SYNC_ANNOTATION_PREFIX+clusterID, "true")
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			createdGateway := &gatewayv1beta1.Gateway{}
			gatewayNamespacedName := types.NamespacedName{Name: gateway.Name, Namespace: controlPlaneNS}

			metadata.AddAnnotation(secret, syncer.MGC_SYNC_ANNOTATION_PREFIX+clusterID, "true")
			Expect(k8sClient.Create(ctx, secret)).To(BeNil())
			createdSecret := &v1.Secret{}
			secretNamespacedName := types.NamespacedName{Name: secret.Name, Namespace: controlPlaneNS}

			// Check it does exist in the control plane
			Expect(k8sClient.Get(ctx, gatewayNamespacedName, createdGateway)).To(BeNil())
			Expect(k8sClient.Get(ctx, secretNamespacedName, createdSecret)).To(BeNil())

			// Check gateway and secret are synced to data plane
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: dataPlaneNS}, &gatewayv1beta1.Gateway{})
				return err == nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: dataPlaneNS}, &v1.Secret{})
				return err == nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Delete secret from control plane
			Expect(k8sClient.Delete(ctx, createdSecret)).To(BeNil())
			// Expect upstream secret to be removed
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: controlPlaneNS}, &v1.Secret{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Expect downstream secret to be removed
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: dataPlaneNS}, &v1.Secret{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Delete gateway from control plane
			Expect(k8sClient.Delete(ctx, createdGateway)).To(BeNil())
			// Expect upstream gateway to be removed
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: controlPlaneNS}, &gatewayv1beta1.Gateway{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Expect downstream gateway to be removed
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: dataPlaneNS}, &gatewayv1beta1.Gateway{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		})
		It("should ignore an annotated gateway and secret for another cluster", func() {
			metadata.AddAnnotation(gateway, syncer.MGC_SYNC_ANNOTATION_PREFIX+"different-cluster-id", "true")
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			createdGateway := &gatewayv1beta1.Gateway{}
			gatewayNamespacedName := types.NamespacedName{Name: gateway.Name, Namespace: controlPlaneNS}

			metadata.AddAnnotation(secret, syncer.MGC_SYNC_ANNOTATION_PREFIX+"different-cluster-id", "true")
			Expect(k8sClient.Create(ctx, secret)).To(BeNil())
			createdSecret := &v1.Secret{}
			secretNamespacedName := types.NamespacedName{Name: secret.Name, Namespace: controlPlaneNS}

			// Check it does exist in the control plane
			Expect(k8sClient.Get(ctx, gatewayNamespacedName, createdGateway)).To(BeNil())
			Expect(k8sClient.Get(ctx, secretNamespacedName, createdSecret)).To(BeNil())

			// Check gateway and secret are never synced to data plane
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: dataPlaneNS}, &gatewayv1beta1.Gateway{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Consistently(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: dataPlaneNS}, &v1.Secret{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		})

		It("should apply correctly annotated json mutations to a gateway and secret", func() {
			metadata.AddAnnotation(gateway, syncer.MGC_SYNC_ANNOTATION_PREFIX+clusterID, "true")
			metadata.AddAnnotation(gateway, mutator.JSONPatchAnnotationPrefix+clusterID, `[
			  {"op": "replace", "path": "/spec/gatewayClassName", "value": "istio"},
			  {"op": "replace", "path": "/spec/listeners/0/name", "value": "test"}
			]`)
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			dataPlaneGateway := &gatewayv1beta1.Gateway{}
			gatewayNamespacedName := types.NamespacedName{Name: gateway.Name, Namespace: dataPlaneNS}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayNamespacedName, dataPlaneGateway)
				if err != nil {
					return false
				}
				ctrl.Log.Info("got dataplane gateway", "yaml", dataPlaneGateway)
				return dataPlaneGateway.Spec.GatewayClassName == "istio" && dataPlaneGateway.Spec.Listeners[0].Name == "test"
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})
	})
})
