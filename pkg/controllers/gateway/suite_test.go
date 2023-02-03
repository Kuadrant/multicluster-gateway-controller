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

package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
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
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&GatewayClassReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
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

var _ = Describe("GatewayClassController", func() {
	Context("testing gatewayclass controller", func() {
		var gatewayclass *gatewayv1beta1.GatewayClass
		BeforeEach(func() {
			gatewayclass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mctc-gw-istio-instance-per-cluster",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mctc-gw-controller",
				},
			}
		})

		AfterEach(func() {
			// Clean up GatewayClasses
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should accept a gatewayclass for this controller", func() {
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Status Accepted
			var condition metav1.Condition
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil && createdGatewayclass.Status.Conditions[0].Status == metav1.ConditionTrue
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			condition = createdGatewayclass.Status.Conditions[0]
			Expect(len(createdGatewayclass.Status.Conditions)).To(BeEquivalentTo(1))
			Expect(condition.Type).To(BeEquivalentTo(gatewayv1beta1.GatewayClassConditionStatusAccepted))
			Expect(condition.Reason).To(BeEquivalentTo(gatewayv1beta1.GatewayClassConditionStatusAccepted))
		})

		It("should NOT accept a gatewayclass for a different controller", func() {
			gatewayclass.Name = "some-other-gatewayclass"
			gatewayclass.Spec.ControllerName = "example.com/some-other-controller"
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Only 1
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(gatewayclassList.Items)).To(BeEquivalentTo(1))

			// Status stays Unknown
			var condition metav1.Condition
			Consistently(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				condition = createdGatewayclass.Status.Conditions[0]
				return condition.Type == string(gatewayv1beta1.GatewayClassConditionStatusAccepted) && condition.Status == metav1.ConditionUnknown
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})

		It("should NOT accept a gatewayclass that is 'unsupported'", func() {
			gatewayclass.Name = "test-class-name-1"
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())

			// Status is false
			var condition metav1.Condition
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				condition = createdGatewayclass.Status.Conditions[0]
				return condition.Type == string(gatewayv1beta1.GatewayClassConditionStatusAccepted) && condition.Status == metav1.ConditionFalse
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			Expect(condition.Reason).To(BeEquivalentTo(gatewayv1beta1.GatewayClassReasonInvalidParameters))
			Expect(condition.Message).To(BeEquivalentTo("Invalid Parameters - Unsupported class name test-class-name-1. Must be one of [mctc-gw-istio-instance-per-cluster]"))
		})
	})
})

var _ = Describe("GatewayController", func() {
	Context("testing gateway controller", func() {
		var gateway *gatewayv1beta1.Gateway
		var gatewayclass *gatewayv1beta1.GatewayClass
		BeforeEach(func() {
			// Create a test GatewayClass for test Gateways to reference
			gatewayclass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mctc-gw-istio-instance-per-cluster",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mctc-gw-controller",
				},
			}
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())

			// Stub Gateway for tests
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-1",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "mctc-gw-istio-instance-per-cluster",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "test-listener-1",
							Port:     8443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
						},
					},
				},
			}
		})

		AfterEach(func() {
			// Clean up GatewayClasses
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}

			// Clean up Gateways
			gatewayList := &gatewayv1beta1.GatewayList{}
			err = k8sClient.List(ctx, gatewayList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gateway := range gatewayList.Items {
				err = k8sClient.Delete(ctx, &gateway)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should create a gateway", func() {
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			createdGateway := &gatewayv1beta1.Gateway{}
			gatewayType := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}

			// Exists
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayType, createdGateway)
				return err == nil
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
		})
	})
})
