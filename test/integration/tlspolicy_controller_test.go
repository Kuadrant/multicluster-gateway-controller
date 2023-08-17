//go:build integration

package integration

import (
	"encoding/json"
	"time"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/tlspolicy"
	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("TLSPolicy", Ordered, func() {

	var testNamespace string
	var gatewayClass *gatewayv1beta1.GatewayClass

	BeforeAll(func() {
		logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
		logger.WithName("tlspolicy_controller_test")
		logf.SetLogger(logger)

		gatewayClass = testBuildGatewayClass("kuadrant-multi-cluster-gateway-instance-per-cluster", "default")
		Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
		Eventually(func() error { // gateway class exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, gatewayClass)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		CreateNamespace(&testNamespace)
	})

	AfterEach(func() {
		gatewayList := &gatewayv1beta1.GatewayList{}
		Expect(k8sClient.List(ctx, gatewayList)).To(BeNil())
		for _, gw := range gatewayList.Items {
			k8sClient.Delete(ctx, &gw)
		}
		policyList := v1alpha1.TLSPolicyList{}
		Expect(k8sClient.List(ctx, &policyList)).To(BeNil())
		for _, policy := range policyList.Items {
			k8sClient.Delete(ctx, &policy)
		}
	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, gatewayClass)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("istio gateway", func() {
		var gateway *gatewayv1beta1.Gateway
		var tlsPolicy *v1alpha1.TLSPolicy
		gwClassName := "istio"

		AfterEach(func() {
			err := k8sClient.Delete(ctx, gateway)
			Expect(err).ToNot(HaveOccurred())
			err = k8sClient.Delete(ctx, tlsPolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("valid target and policy", func() {

			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPListener("test.example.com").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
					WithTargetGateway(gateway.Name).TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should have ready status", func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy); err != nil {
						return false
					}
					return meta.IsStatusConditionTrue(tlsPolicy.Status.Conditions, string(conditions.ConditionTypeReady))
				}, time.Second*15, time.Second).Should(BeTrue())
			})

			It("should set gateway back reference", func() {
				existingGateway := &gatewayv1beta1.Gateway{}
				policyBackRefValue := testNamespace + "/" + tlsPolicy.Name
				refs, _ := json.Marshal([]client.ObjectKey{{Name: tlsPolicy.Name, Namespace: testNamespace}})
				policiesBackRefValue := string(refs)
				Eventually(func() map[string]string {
					// Check gateway back references
					err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
					// must exist
					Expect(err).ToNot(HaveOccurred())
					return existingGateway.GetAnnotations()
				}, time.Second*5, time.Second).Should(HaveKeyWithValue(TLSPolicyBackRefAnnotation, policyBackRefValue))
				Eventually(func() map[string]string {
					// Check gateway back references
					err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
					// must exist
					Expect(err).ToNot(HaveOccurred())
					return existingGateway.GetAnnotations()
				}, time.Second*5, time.Second).Should(HaveKeyWithValue(TLSPoliciesBackRefAnnotation, policiesBackRefValue))
			})

		})

		Context("with http listener", func() {

			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPListener("test.example.com").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
					WithTargetGateway(gateway.Name).TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should not create any certificates", func() {
				Consistently(func() []certmanv1.Certificate {
					certList := &certmanv1.CertificateList{}
					err := k8sClient.List(ctx, certList, &client.ListOptions{Namespace: testNamespace})
					Expect(err).ToNot(HaveOccurred())
					return certList.Items
				}, time.Second*10, time.Second).Should(BeEmpty())
			})

		})

		Context("with https listener", func() {

			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPSListener("test.example.com", "test-tls-secret").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
					WithTargetGateway(gateway.Name).TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create tls certificate", func() {
				Eventually(func() []certmanv1.Certificate {
					certList := &certmanv1.CertificateList{}
					err := k8sClient.List(ctx, certList, &client.ListOptions{Namespace: testNamespace})
					Expect(err).ToNot(HaveOccurred())
					return certList.Items
				}, time.Second*10, time.Second).Should(HaveLen(1))

				cert1 := &certmanv1.Certificate{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-tls-secret", Namespace: testNamespace}, cert1)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with multiple https listener", func() {

			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPSListener("test1.example.com", "test-tls-secret").
					WithHTTPSListener("test2.example.com", "test-tls-secret").
					WithHTTPSListener("test3.example.com", "test2-tls-secret").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
					WithTargetGateway(gateway.Name).TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create tls certificates", func() {
				Eventually(func() []certmanv1.Certificate {
					certList := &certmanv1.CertificateList{}
					err := k8sClient.List(ctx, certList, &client.ListOptions{Namespace: testNamespace})
					Expect(err).ToNot(HaveOccurred())
					return certList.Items
				}, time.Second*10, time.Second).Should(HaveLen(2))

				cert1 := &certmanv1.Certificate{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-tls-secret", Namespace: testNamespace}, cert1)
				Expect(err).ToNot(HaveOccurred())

				cert2 := &certmanv1.Certificate{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "test2-tls-secret", Namespace: testNamespace}, cert2)
				Expect(err).ToNot(HaveOccurred())
			})
		})

	})

})
