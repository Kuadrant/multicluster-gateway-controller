//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
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
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnspolicy"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
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
			SecretRef: &v1alpha1.SecretRef{
				Name:      "secretname",
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
			LoadBalancing: &v1alpha1.LoadBalancingSpec{
				Weighted: &v1alpha1.LoadBalancingWeighted{
					DefaultWeight: 120,
				},
			},
		},
	}
}

func testBuildDNSPolicyWithGeo(policyName, gwName, ns string) *v1alpha1.DNSPolicy {
	typedNamespace := gatewayv1beta1.Namespace(ns)
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
			LoadBalancing: &v1alpha1.LoadBalancingSpec{
				Weighted: &v1alpha1.LoadBalancingWeighted{
					DefaultWeight: 120,
				},
				Geo: &v1alpha1.LoadBalancingGeo{
					DefaultGeo: "IE",
				},
			},
		},
	}
}

var _ = Describe("DNSPolicy", Ordered, func() {

	var gatewayClass *gatewayv1beta1.GatewayClass
	var managedZone *v1alpha1.ManagedZone
	var testNamespace string

	BeforeAll(func() {
		gatewayClass = testBuildGatewayClass("kuadrant-multi-cluster-gateway-instance-per-cluster-dns", "default")
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

	AfterAll(func() {
		err := k8sClient.Delete(ctx, gatewayClass)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("gateway placed", func() {
		var gateway *gatewayv1beta1.Gateway
		var lbHash string

		BeforeEach(func() {
			gateway = testBuildGateway(TestPlacedGatewayName, gatewayClass.Name, TestAttachedRouteName, testNamespace)
			lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() bool { //gateway exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		Context("weighted dnspolicy", func() {
			var dnsPolicy *v1alpha1.DNSPolicy

			BeforeEach(func() {
				dnsPolicy = testBuildDNSPolicyWithHealthCheck("test-dns-policy", TestPlacedGatewayName, testNamespace)
				Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
				Eventually(func() bool { //dns policy exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
						return false
					}
					return true
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			})

			It("should create a dns record", func() {
				createdDNSRecord := &v1alpha1.DNSRecord{}
				expectedEndpoints := []*v1alpha1.Endpoint{
					{
						DNSName: "test.example.com",
						Targets: []string{
							"lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "",
						RecordTTL:     300,
					},
					{
						DNSName: "lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"default.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     300,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "geo-country-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "16z1l1.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"172.0.0.3",
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "default.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"16z1l1.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "16z1l1.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
				}
				Eventually(func() bool { // DNS record exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: TestAttachedRouteName, Namespace: dnsPolicy.Namespace}, createdDNSRecord); err != nil {
						return false
					}
					return len(createdDNSRecord.Spec.Endpoints) == 4
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
				Expect(createdDNSRecord.Spec.ManagedZoneRef.Name).To(Equal("example.com"))
				Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(4))
				Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
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
				}, time.Second*5, time.Second).ShouldNot(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))
			})
		})

		Context("geo dnspolicy", func() {
			var dnsPolicy *v1alpha1.DNSPolicy

			BeforeEach(func() {
				dnsPolicy = testBuildDNSPolicyWithGeo("test-dns-policy", TestPlacedGatewayName, testNamespace)
				Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
				Eventually(func() bool { //dns policy exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
						return false
					}
					return true
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			})

			It("should create a dns record", func() {
				createdDNSRecord := &v1alpha1.DNSRecord{}
				expectedEndpoints := []*v1alpha1.Endpoint{
					{
						DNSName: "test.example.com",
						Targets: []string{
							"lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "",
						RecordTTL:     300,
					},
					{
						DNSName: "lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"ie.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "IE",
						RecordTTL:     300,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "geo-country-code",
								Value: "IE",
							},
						},
					},
					{
						DNSName: "lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"ie.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     300,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "geo-country-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "16z1l1.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"172.0.0.3",
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "ie.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"16z1l1.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "16z1l1.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
				}
				Eventually(func() bool { // DNS record exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: TestAttachedRouteName, Namespace: dnsPolicy.Namespace}, createdDNSRecord); err != nil {
						return false
					}
					return len(createdDNSRecord.Spec.Endpoints) == 5
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
				Expect(createdDNSRecord.Spec.ManagedZoneRef.Name).To(Equal("example.com"))
				Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(5))
				Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
			})
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

		AfterAll(func() {
			err := k8sClient.Delete(ctx, gateway)
			Expect(err).ToNot(HaveOccurred())
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
