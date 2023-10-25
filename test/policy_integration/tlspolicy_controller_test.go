//go:build integration

package policy_integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	var issuer *certmanv1.Issuer

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
		issuer = NewTestIssuer("testissuer", testNamespace)
		Expect(k8sClient.Create(ctx, issuer)).To(BeNil())
		Eventually(func() error { //issuer exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: issuer.Name, Namespace: issuer.Namespace}, issuer)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		gatewayList := &gatewayv1beta1.GatewayList{}
		Expect(k8sClient.List(ctx, gatewayList)).To(BeNil())
		for _, gw := range gatewayList.Items {
			err := k8sClient.Delete(ctx, &gw)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
		policyList := v1alpha1.TLSPolicyList{}
		Expect(k8sClient.List(ctx, &policyList)).To(BeNil())
		for _, policy := range policyList.Items {
			err := k8sClient.Delete(ctx, &policy)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
		issuerList := certmanv1.IssuerList{}
		Expect(k8sClient.List(ctx, &issuerList)).To(BeNil())
		for _, issuer := range issuerList.Items {
			err := k8sClient.Delete(ctx, &issuer)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, gatewayClass)
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

	})

	Context("invalid target", func() {
		var gateway *gatewayv1beta1.Gateway
		var tlsPolicy *v1alpha1.TLSPolicy
		gwClassName := "istio"

		BeforeEach(func() {
			tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
				WithTargetGateway("test-gateway").
				WithIssuer("testissuer", certmanv1.IssuerKind, "cert-manager.io").TLSPolicy
			Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
			Eventually(func() error { //tls policy exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, tlsPolicy)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have ready condition with status false and correct reason", func() {
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy); err != nil {
					return err
				}
				readyCond := meta.FindStatusCondition(tlsPolicy.Status.Conditions, string(conditions.ConditionTypeReady))
				if readyCond == nil {
					return fmt.Errorf("expected tlsPolicy to have %s condition, got none",
						string(conditions.ConditionTypeReady))
				}
				if readyCond.Status != metav1.ConditionFalse {
					return fmt.Errorf("expected tlsPolicy %s condition to have status %s, got %s",
						string(conditions.ConditionTypeReady), metav1.ConditionFalse, readyCond.Status)
				}
				if readyCond.Reason != string(conditions.PolicyReasonTargetNotFound) {
					return fmt.Errorf("expected tlsPolicy %s condition to have reason %s, got %s",
						string(conditions.ConditionTypeReady), string(conditions.PolicyReasonTargetNotFound), readyCond.Reason)
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should have ready condition with status true", func() {
			By("creating a valid Gateway")

			gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
				WithHTTPListener("test.example.com").Gateway
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy); err != nil {
					return err
				}
				if !meta.IsStatusConditionTrue(tlsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
					return fmt.Errorf("expected tlsPolicy %s condition to have status %s ", string(conditions.ConditionTypeReady), metav1.ConditionTrue)
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

	})

	Context("istio gateway", func() {
		var gateway *gatewayv1beta1.Gateway
		var tlsPolicy *v1alpha1.TLSPolicy
		gwClassName := "istio"

		AfterEach(func() {
			if gateway != nil {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, gateway))).ToNot(HaveOccurred())
			}
			if tlsPolicy != nil {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, tlsPolicy))).ToNot(HaveOccurred())
			}
		})

		Context("gateway with no TLS Policy and multiple listeners", func() {
			manualSecretName := "manual-tls-secret"
			manualListenerName := "manual-test.example.com"
			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPSListener(manualListenerName, manualSecretName).
					WithHTTPSListener("test2.example.com", "test2-tls-secret").
					WithHTTPSListener("*.example.com", "wildcard-test-tls-secret").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})
			AfterEach(func() {
				err := k8sClient.Delete(ctx, gateway)
				Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
			})
			It("should not be programmed", func() {
				Consistently(func() error {
					freshGW := &gatewayv1beta1.Gateway{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, freshGW)
					if err != nil {
						return err
					}
					if freshGW.Status.Conditions == nil {
						return nil
					}
					for _, condition := range freshGW.Status.Conditions {
						if condition.Type == string(gatewayv1beta1.GatewayConditionProgrammed) {
							if strings.ToLower(string(condition.Status)) == "true" {
								return fmt.Errorf("expected programmed status false, got true")
							}
						}
					}
					return nil
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
			})
		})

		Context("valid target, issuer and policy", func() {
			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPListener("test.example.com").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
					WithTargetGateway(gateway.Name).
					WithIssuer("testissuer", certmanv1.IssuerKind, "cert-manager.io").TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should have ready status", func() {
				Eventually(func() error {
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy); err != nil {
						return err
					}
					if !meta.IsStatusConditionTrue(tlsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
						return fmt.Errorf("expected tlsPolicy status condition to be %s", string(conditions.ConditionTypeReady))
					}
					return nil
				}, time.Second*15, time.Second).Should(BeNil())
			})

			It("should set gateway back reference", func() {
				existingGateway := &gatewayv1beta1.Gateway{}
				policyBackRefValue := testNamespace + "/" + tlsPolicy.Name
				refs, _ := json.Marshal([]client.ObjectKey{{Name: tlsPolicy.Name, Namespace: testNamespace}})
				policiesBackRefValue := string(refs)
				Eventually(func() error {
					// Check gateway back references
					err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
					Expect(err).ToNot(HaveOccurred())
					annotations := existingGateway.GetAnnotations()
					if annotations == nil {
						return fmt.Errorf("existingGateway annotations should not be nil")
					}
					if _, ok := annotations[TLSPolicyBackRefAnnotation]; !ok {
						return fmt.Errorf("existingGateway annotations do not have annotation %s", TLSPolicyBackRefAnnotation)
					}
					if annotations[TLSPolicyBackRefAnnotation] != policyBackRefValue {
						return fmt.Errorf("existingGateway annotations[%s] does not have expected value", TLSPolicyBackRefAnnotation)
					}
					return nil
				}, time.Second*5, time.Second).Should(BeNil())
				Eventually(func() error {
					// Check gateway back references
					err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
					Expect(err).ToNot(HaveOccurred())
					annotations := existingGateway.GetAnnotations()
					if annotations == nil {
						return fmt.Errorf("existingGateway annotations should not be nil")
					}
					if _, ok := annotations[TLSPoliciesBackRefAnnotation]; !ok {
						return fmt.Errorf("existingGateway annotations do not have annotation %s", TLSPoliciesBackRefAnnotation)
					}
					if annotations[TLSPoliciesBackRefAnnotation] != policiesBackRefValue {
						return fmt.Errorf("existingGateway annotations[%s] does not have expected value", TLSPoliciesBackRefAnnotation)
					}
					return nil
				}, time.Second*5, time.Second).Should(BeNil())
			})

			It("should set policy affected condition in gateway status", func() {
				Eventually(func() error {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(gateway), gateway); err != nil {
						return err
					}

					policyAffectedCond := meta.FindStatusCondition(gateway.Status.Conditions, string(TLSPolicyAffected))
					if policyAffectedCond == nil {
						return fmt.Errorf("policy affected conditon expected but not found")
					}
					if policyAffectedCond.ObservedGeneration != gateway.Generation {
						return fmt.Errorf("expected policy affected cond generation to be %d but got %d", gateway.Generation, policyAffectedCond.ObservedGeneration)
					}
					if !meta.IsStatusConditionTrue(gateway.Status.Conditions, string(TLSPolicyAffected)) {
						return fmt.Errorf("expected gateway status condition %s to be True", TLSPolicyAffected)
					}

					return nil
				}, time.Second*15, time.Second).Should(BeNil())
			})

		})

		Context("valid target, clusterissuer and policy", func() {
			var clusterIssuer *certmanv1.ClusterIssuer

			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPListener("test.example.com").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
					WithTargetGateway(gateway.Name).
					WithIssuer("testclusterissuer", certmanv1.ClusterIssuerKind, "cert-manager.io").TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				clusterIssuer = NewTestClusterIssuer("testclusterissuer")
				Expect(k8sClient.Create(ctx, clusterIssuer)).To(BeNil())
				Eventually(func() error { //clusterIssuer exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: clusterIssuer.Name}, clusterIssuer)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should have ready status", func() {
				Eventually(func() error {
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy); err != nil {
						return err
					}
					if !meta.IsStatusConditionTrue(tlsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
						return fmt.Errorf("expected tlsPolicy status condition to be %s", string(conditions.ConditionTypeReady))
					}
					return nil
				}, time.Second*15, time.Second).Should(BeNil())
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
					WithTargetGateway(gateway.Name).
					WithIssuer("testissuer", certmanv1.IssuerKind, "cert-manager.io").TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should not create any certificates when TLS is not present", func() {
				Consistently(func() []certmanv1.Certificate {
					certList := &certmanv1.CertificateList{}
					err := k8sClient.List(ctx, certList, &client.ListOptions{Namespace: testNamespace})
					Expect(err).ToNot(HaveOccurred())
					return certList.Items
				}, time.Second*10, time.Second).Should(BeEmpty())
			})

			It("should create certificate when TLS is present", func() {
				certNS := gatewayv1beta1.Namespace(testNamespace)
				patch := client.MergeFrom(gateway.DeepCopy())
				gateway.Spec.Listeners[0].TLS = &gatewayv1beta1.GatewayTLSConfig{
					Mode: Pointer(gatewayv1beta1.TLSModeTerminate),
					CertificateRefs: []gatewayv1beta1.SecretObjectReference{
						{
							Name:      "test-tls-secret",
							Namespace: &certNS,
						},
					},
				}
				Expect(k8sClient.Patch(ctx, gateway, patch)).To(BeNil())
				Eventually(func() error {
					cert := &certmanv1.Certificate{}
					return k8sClient.Get(ctx, client.ObjectKey{Name: "test-tls-secret", Namespace: testNamespace}, cert)
				}, time.Second*10, time.Second).Should(BeNil())
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
					WithTargetGateway(gateway.Name).
					WithIssuer("testissuer", certmanv1.IssuerKind, "cert-manager.io").TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create tls certificate", func() {
				Eventually(func() error {
					certList := &certmanv1.CertificateList{}
					err := k8sClient.List(ctx, certList, &client.ListOptions{Namespace: testNamespace})
					Expect(err).ToNot(HaveOccurred())
					if len(certList.Items) != 1 {
						return fmt.Errorf("expected certificate List to be 1")
					}
					return nil
				}, time.Second*10, time.Second).Should(BeNil())

				cert1 := &certmanv1.Certificate{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-tls-secret", Namespace: testNamespace}, cert1)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with multiple https listener and some shared secrets", func() {
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
					WithTargetGateway(gateway.Name).
					WithIssuer("testissuer", certmanv1.IssuerKind, "cert-manager.io").TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create tls certificates", func() {
				Eventually(func() error {
					certList := &certmanv1.CertificateList{}
					err := k8sClient.List(ctx, certList, &client.ListOptions{Namespace: testNamespace})
					Expect(err).ToNot(HaveOccurred())
					if len(certList.Items) != 2 {
						return fmt.Errorf("expected CertificateList to be 2")
					}
					return nil
				}, time.Second*30, time.Second).Should(BeNil())

				cert1 := &certmanv1.Certificate{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-tls-secret", Namespace: testNamespace}, cert1)
				Expect(err).ToNot(HaveOccurred())

				Expect(cert1.Spec.DNSNames).To(ConsistOf("test1.example.com", "test2.example.com"))

				cert2 := &certmanv1.Certificate{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "test2-tls-secret", Namespace: testNamespace}, cert2)
				Expect(err).ToNot(HaveOccurred())

				Expect(cert2.Spec.DNSNames).To(ConsistOf("test3.example.com"))
			})
		})

		Context("with multiple https listener", func() {
			BeforeEach(func() {
				gateway = NewTestGateway("test-gateway", gwClassName, testNamespace).
					WithHTTPSListener("test1.example.com", "test1-tls-secret").
					WithHTTPSListener("test2.example.com", "test2-tls-secret").
					WithHTTPSListener("test3.example.com", "test3-tls-secret").Gateway
				Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
				Eventually(func() error { //gateway exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				tlsPolicy = NewTestTLSPolicy("test-tls-policy", testNamespace).
					WithTargetGateway(gateway.Name).
					WithIssuer("testissuer", certmanv1.IssuerKind, "cert-manager.io").TLSPolicy
				Expect(k8sClient.Create(ctx, tlsPolicy)).To(BeNil())
				Eventually(func() error { //tls policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: tlsPolicy.Name, Namespace: tlsPolicy.Namespace}, tlsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create tls certificates", func() {
				Eventually(func() error {
					certList := &certmanv1.CertificateList{}
					err := k8sClient.List(ctx, certList, &client.ListOptions{Namespace: testNamespace})
					Expect(err).ToNot(HaveOccurred())
					if len(certList.Items) != 3 {
						return fmt.Errorf("expected CertificateList to be 3")
					}
					return nil
				}, time.Second*30, time.Second).Should(BeNil())

				cert1 := &certmanv1.Certificate{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test1-tls-secret", Namespace: testNamespace}, cert1)
				Expect(err).ToNot(HaveOccurred())

				Expect(cert1.Spec.DNSNames).To(ConsistOf("test1.example.com"))

				cert2 := &certmanv1.Certificate{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "test2-tls-secret", Namespace: testNamespace}, cert2)
				Expect(err).ToNot(HaveOccurred())

				Expect(cert2.Spec.DNSNames).To(ConsistOf("test2.example.com"))

				cert3 := &certmanv1.Certificate{}
				err = k8sClient.Get(ctx, client.ObjectKey{Name: "test3-tls-secret", Namespace: testNamespace}, cert3)
				Expect(err).ToNot(HaveOccurred())

				Expect(cert3.Spec.DNSNames).To(ConsistOf("test3.example.com"))
			})
			It("should delete tls certificate when listener is removed", func() {
				//confirm all expected certificates are present
				Eventually(func() error {
					certificateList := &certmanv1.CertificateList{}
					Expect(k8sClient.List(ctx, certificateList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
					if len(certificateList.Items) != 3 {
						return fmt.Errorf("expected 3 certificates, found: %v", len(certificateList.Items))
					}
					return nil
				}, time.Second*60, time.Second).Should(BeNil())

				//remove a listener
				patch := client.MergeFrom(gateway.DeepCopy())
				gateway.Spec.Listeners = gateway.Spec.Listeners[1:]
				Expect(k8sClient.Patch(ctx, gateway, patch)).To(BeNil())

				//confirm a certificate has been deleted
				Eventually(func() error {
					certificateList := &certmanv1.CertificateList{}
					Expect(k8sClient.List(ctx, certificateList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
					if len(certificateList.Items) != 2 {
						return fmt.Errorf("expected 2 certificates, found: %v", len(certificateList.Items))
					}
					return nil
				}, time.Second*120, time.Second).Should(BeNil())
			})
			It("should delete all tls certificates when tls policy is removed even if gateway is already removed", func() {
				//confirm all expected certificates are present
				Eventually(func() error {
					certificateList := &certmanv1.CertificateList{}
					Expect(k8sClient.List(ctx, certificateList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
					if len(certificateList.Items) != 3 {
						return fmt.Errorf("expected 3 certificates, found: %v", len(certificateList.Items))
					}
					return nil
				}, time.Second*10, time.Second).Should(BeNil())

				// delete the gateway
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, gateway))).ToNot(HaveOccurred())

				//delete the tls policy
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, tlsPolicy))).ToNot(HaveOccurred())

				//confirm all certificates have been deleted
				Eventually(func() error {
					certificateList := &certmanv1.CertificateList{}
					Expect(k8sClient.List(ctx, certificateList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
					if len(certificateList.Items) != 0 {
						return fmt.Errorf("expected 0 certificates, found: %v", len(certificateList.Items))
					}
					return nil
				}, time.Second*60, time.Second).Should(BeNil())
			})
		})

	})
})
