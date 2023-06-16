//go:build integration

package dnspolicy

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

func testBuildManagedZone(domainName, ns string) *v1alpha1.ManagedZone {
	return &v1alpha1.ManagedZone{
		ObjectMeta: metav1.ObjectMeta{
			Name:      domainName,
			Namespace: ns,
		},
		Spec: v1alpha1.ManagedZoneSpec{
			ID:          "1234",
			DomainName:  domainName,
			Description: domainName,
			ProviderRef: &v1alpha1.ProviderRef{
				Name:      "secretName",
				Namespace: ns,
			},
		},
	}
}

func testBuildGatewayClass(gwClassName, ns string) *gatewayv1beta1.GatewayClass {
	return &gatewayv1beta1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwClassName,
			Namespace: ns,
		},
		Spec: gatewayv1beta1.GatewayClassSpec{
			ControllerName: "kuadrant.io/mctc-gw-controller",
		},
	}
}

func testBuildGateway(gwName, gwClassName, hostname, ns string) *gatewayv1beta1.Gateway {
	typedHostname := gatewayv1beta1.Hostname(hostname)
	return &gatewayv1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwName,
			Namespace: ns,
		},
		Spec: gatewayv1beta1.GatewaySpec{
			GatewayClassName: gatewayv1beta1.ObjectName(gwClassName),
			Listeners: []gatewayv1beta1.Listener{
				{
					Name:     gatewayv1beta1.SectionName(hostname),
					Hostname: &typedHostname,
					Port:     gatewayv1beta1.PortNumber(80),
					Protocol: gatewayv1beta1.HTTPProtocolType,
				},
			},
		},
	}
}

func testBuildDNSPolicyWithHealthCheck(policyName, gwName, ns string) *v1alpha1.DNSPolicy {
	typedNamespace := gatewayv1beta1.Namespace(ns)
	protocol := v1alpha1.HttpProtocol
	return &v1alpha1.DNSPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: ns,
		},
		Spec: v1alpha1.DNSPolicySpec{
			TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
				Group:     "gateway.networking.k8s.io",
				Kind:      "Gateway",
				Name:      gatewayv1beta1.ObjectName(gwName),
				Namespace: &typedNamespace,
			},
			HealthCheck: &v1alpha1.HealthCheckSpec{
				Endpoint: "/",
				Protocol: &protocol,
			},
		},
	}
}

var _ = Describe("DNSPolicy", Ordered, func() {

	var gatewayClass *gatewayv1beta1.GatewayClass
	var managedZone *v1alpha1.ManagedZone
	var testNamespace string

	BeforeAll(func() {
		gatewayClass = testBuildGatewayClass("kuadrant-multi-cluster-gateway-instance-per-cluster", "default")
		Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
		Eventually(func() bool { // gateway class exists
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, gatewayClass); err != nil {
				return false
			}
			return true

		}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
	})

	BeforeEach(func() {
		CreateNamespace(&testNamespace)

		managedZone = testBuildManagedZone("example.com", testNamespace)
		Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())
		Eventually(func() bool { // managed zone exists
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: managedZone.Name, Namespace: managedZone.Namespace}, managedZone); err != nil {
				return false
			}
			return true
		}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
	})

	Context("gateway placed", func() {
		var gateway *gatewayv1beta1.Gateway
		var dnsPolicy *v1alpha1.DNSPolicy

		BeforeEach(func() {
			gateway = testBuildGateway(TestPlacedGatewayName, gatewayClass.Name, TestAttachedRouteName, testNamespace)
			dnsPolicy = testBuildDNSPolicyWithHealthCheck("test-dns-policy", TestPlacedGatewayName, testNamespace)

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())

			Eventually(func() bool { //gateway exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Eventually(func() bool { //dns policy exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should create dns record", func() {
			createdDNSRecord := &v1alpha1.DNSRecord{}
			Eventually(func() bool { // DNS record exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: TestAttachedRouteName, Namespace: dnsPolicy.Namespace}, createdDNSRecord); err != nil {
					return false
				}

				if len(createdDNSRecord.Spec.Endpoints) != 1 && createdDNSRecord.Spec.Endpoints[0].DNSName != TestAttachedRouteAddress {
					return false
				}

				prop, ok := createdDNSRecord.Spec.Endpoints[0].GetProviderSpecificProperty("weight")
				if !ok || prop.Value != "120" {
					return false
				}

				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should have ready status", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return false
				}

				return meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, conditions.ConditionTypeReady)
			}, time.Second*15, time.Second).Should(BeTrue())
		})

		It("should have health check status", func() {
			Eventually(func() bool { // DNS Policy has health check status
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return false
				}

				if dnsPolicy.Status.HealthCheck == nil || dnsPolicy.Status.HealthCheck.Conditions == nil {
					return false
				}
				return len(dnsPolicy.Status.HealthCheck.Conditions) > 0
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should set gateway back reference", func() {
			existingGateway := &gatewayv1beta1.Gateway{}
			policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
			refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
			policiesBackRefValue := string(refs)

			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))
		})

		It("should remove gateway back reference on policy deletion", func() {
			existingGateway := &gatewayv1beta1.Gateway{}
			policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
			refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
			policiesBackRefValue := string(refs)

			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))

			Expect(k8sClient.Delete(ctx, dnsPolicy)).To(BeNil())

			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).ShouldNot(HaveKey(DNSPolicyBackRefAnnotation))
			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).ShouldNot(HaveKey(DNSPolicyBackRefAnnotation))

		})

	})

	Context("gateway not placed", func() {
		var gateway *gatewayv1beta1.Gateway
		var dnsPolicy *v1alpha1.DNSPolicy
		testGatewayName := "test-not-placed-gateway"

		BeforeEach(func() {
			gateway = testBuildGateway(testGatewayName, gatewayClass.Name, TestAttachedRouteName, testNamespace)
			dnsPolicy = testBuildDNSPolicyWithHealthCheck("test-dns-policy", testGatewayName, testNamespace)

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())

			Eventually(func() bool { //gateway exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Eventually(func() bool { //dns policy exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should not create a dns record", func() {
			Consistently(func() []v1alpha1.DNSRecord { // DNS record exists
				dnsRecords := v1alpha1.DNSRecordList{}
				err := k8sClient.List(ctx, &dnsRecords, client.InNamespace(dnsPolicy.GetNamespace()))
				Expect(err).ToNot(HaveOccurred())
				return dnsRecords.Items
			}, time.Second*15, time.Second).Should(BeEmpty())
		})

		It("should have ready status", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return false
				}

				return meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, conditions.ConditionTypeReady)
			}, time.Second*15, time.Second).Should(BeTrue())
		})

		It("should set gateway back reference", func() {
			existingGateway := &gatewayv1beta1.Gateway{}
			policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
			refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
			policiesBackRefValue := string(refs)
			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))
		})

	})

})
