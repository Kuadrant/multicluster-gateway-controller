//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnspolicy"
	mgcgateway "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
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

func testBuildGateway(gwName, gwClassName, hostname, ns, dnspolicy string) *gatewayv1beta1.Gateway {
	typedHostname := gatewayv1beta1.Hostname(hostname)
	wildcardHost := gatewayv1beta1.Hostname(TestWildCardListenerHost)
	return &gatewayv1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwName,
			Namespace: ns,
			Annotations: map[string]string{
				DNSPoliciesBackRefAnnotation: fmt.Sprintf("[{\"Namespace\":\"%s\",\"Name\":\"%s\"}]", ns, dnspolicy),
			},
			Labels: map[string]string{
				"cluster.open-cluster-management.io/placement": "GatewayControllerTest",
			},
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
				{
					Name:     gatewayv1beta1.SectionName(TestWildCardListenerName),
					Hostname: &wildcardHost,
					Port:     gatewayv1beta1.PortNumber(80),
					Protocol: gatewayv1beta1.HTTPProtocolType,
				},
			},
		},
	}
}

func testBuildDNSPolicyWithHealthCheck(policyName, gwName, ns string, threshold *int) *v1alpha1.DNSPolicy {
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
				Endpoint:         "/",
				Protocol:         &protocol,
				FailureThreshold: threshold,
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
		logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
		logger.WithName("dnspolicy_controller_test")
		logf.SetLogger(logger)

		gatewayClass = testBuildGatewayClass("kuadrant-multi-cluster-gateway-instance-per-cluster", "default")
		Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
		Eventually(func() error { // gateway class exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, gatewayClass)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		CreateNamespace(&testNamespace)

		managedZone = testBuildManagedZone("example.com", testNamespace)
		Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())
		Eventually(func() error { // managed zone exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: managedZone.Name, Namespace: managedZone.Namespace}, managedZone)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

	})

	AfterEach(func() {
		gatewayList := &gatewayv1beta1.GatewayList{}
		Expect(k8sClient.List(ctx, gatewayList)).To(BeNil())
		for _, gw := range gatewayList.Items {
			k8sClient.Delete(ctx, &gw)
		}
		policyList := v1alpha1.DNSPolicyList{}
		Expect(k8sClient.List(ctx, &policyList)).To(BeNil())
		for _, policy := range policyList.Items {
			k8sClient.Delete(ctx, &policy)
		}
	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, gatewayClass)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("gateway placed", func() {
		var gateway *gatewayv1beta1.Gateway
		var lbHash, dnsRecordName, wildcardDNSRecordName string

		BeforeEach(func() {
			gateway = testBuildGateway(TestPlacedGatewayName, gatewayClass.Name, TestAttachedRouteName, testNamespace, "test-dns-policy")
			lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))
			dnsRecordName = fmt.Sprintf("%s-%s", TestPlacedGatewayName, TestAttachedRouteName)
			wildcardDNSRecordName = fmt.Sprintf("%s-%s", TestPlacedGatewayName, TestWildCardListenerName)
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, gateway)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("weighted dnspolicy", func() {
			var dnsPolicy *v1alpha1.DNSPolicy

			BeforeEach(func() {
				dnsPolicy = testBuildDNSPolicyWithHealthCheck("test-dns-policy", TestPlacedGatewayName, testNamespace, nil)
				Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
				Eventually(func() error { //dns policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
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
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "s07c46.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							TestAttachedRouteAddressOne,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "default.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"s07c46.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "s07c46.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName: "default.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"2w705o.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "2w705o.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName: "2w705o.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							TestAttachedRouteAddressTwo,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
				}
				Eventually(func() error { // DNS record exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsRecordName, Namespace: testNamespace}, createdDNSRecord); err != nil {
						return err
					}
					if len(createdDNSRecord.Spec.Endpoints) != len(expectedEndpoints) {
						return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", len(expectedEndpoints), len(createdDNSRecord.Spec.Endpoints))
					}
					return nil
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				Expect(createdDNSRecord.Spec.ManagedZoneRef.Name).To(Equal("example.com"))
				Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(6))
				Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
			})
			It("should create a wildcard dns record", func() {
				wildcardDNSRecord := &v1alpha1.DNSRecord{}
				expectedEndpoints := []*v1alpha1.Endpoint{
					{
						DNSName: TestWildCardListenerHost,
						Targets: []string{
							"lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "",
						RecordTTL:     300,
					},
					{
						DNSName: "lb-" + lbHash + ".example.com",
						Targets: []string{
							"default.lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     300,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "s07c46.lb-" + lbHash + ".example.com",
						Targets: []string{
							TestAttachedRouteAddressOne,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "default.lb-" + lbHash + ".example.com",
						Targets: []string{
							"s07c46.lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "s07c46.lb-" + lbHash + ".example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName: "2w705o.lb-" + lbHash + ".example.com",
						Targets: []string{
							TestAttachedRouteAddressTwo,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "default.lb-" + lbHash + ".example.com",
						Targets: []string{
							"2w705o.lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "2w705o.lb-" + lbHash + ".example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
				}
				Eventually(func() error { // DNS record exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: wildcardDNSRecordName, Namespace: testNamespace}, wildcardDNSRecord); err != nil {
						return err
					}
					if len(wildcardDNSRecord.Spec.Endpoints) != len(expectedEndpoints) {
						return fmt.Errorf("expected %v wildcard endpoints in DNSRecord, got %v", len(expectedEndpoints), len(wildcardDNSRecord.Spec.Endpoints))
					}
					return nil
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				Expect(wildcardDNSRecord.Spec.ManagedZoneRef.Name).To(Equal("example.com"))
				Expect(wildcardDNSRecord.Spec.Endpoints).To(HaveLen(6))
				Expect(wildcardDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
				Expect(expectedEndpoints).Should(ContainElements(wildcardDNSRecord.Spec.Endpoints))
			})

			It("should have correct status", func() {
				Eventually(func() error {
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
						return err
					}
					if !meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
						return fmt.Errorf("expected status condition %s to be True", conditions.ConditionTypeReady)
					}
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(gateway), gateway); err != nil {
						return err
					}

					policyAffectedCond := meta.FindStatusCondition(gateway.Status.Conditions, string(DNSPolicyAffected))
					if policyAffectedCond == nil {
						return fmt.Errorf("policy affected conditon expected but not found")
					}
					if policyAffectedCond.ObservedGeneration != gateway.Generation {
						return fmt.Errorf("expected policy affected cond generation to be %d but got %d", gateway.Generation, policyAffectedCond.ObservedGeneration)
					}
					if !meta.IsStatusConditionTrue(gateway.Status.Conditions, string(DNSPolicyAffected)) {
						return fmt.Errorf("expected gateway status condition %s to be True", DNSPolicyAffected)
					}

					return nil
				}, time.Second*15, time.Second).Should(BeNil())
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

			It("should remove dns records when listener removed", func() {
				//get the gateway and remove the listeners

				Eventually(func() error {
					existingGateway := &gatewayv1beta1.Gateway{}
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(gateway), existingGateway); err != nil {
						return err
					}
					newListeners := []gatewayv1beta1.Listener{}
					for _, existing := range existingGateway.Spec.Listeners {
						if existing.Name == TestWildCardListenerName {
							newListeners = append(newListeners, existing)
						}
					}

					existingGateway.Spec.Listeners = newListeners
					rec := &v1alpha1.DNSRecord{}
					if err := k8sClient.Update(ctx, existingGateway, &client.UpdateOptions{}); err != nil {
						return err
					}
					//dns record should be removed for non wildcard
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsRecordName, Namespace: testNamespace}, rec); err != nil && !k8serrors.IsNotFound(err) {
						return err
					}
					return k8sClient.Get(ctx, client.ObjectKey{Name: wildcardDNSRecordName, Namespace: testNamespace}, rec)
				}, time.Second*10, time.Second).Should(BeNil())
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

				//finalizer should exist
				Eventually(func() bool {
					existingDNSPolicy := &v1alpha1.DNSPolicy{}
					err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: testNamespace}, existingDNSPolicy)
					// must exist
					Expect(err).ToNot(HaveOccurred())
					return metadata.HasFinalizer(existingDNSPolicy, DNSPolicyFinalizer)
				}, time.Second*5, time.Second).Should(BeTrue())

				Expect(k8sClient.Delete(ctx, dnsPolicy)).To(BeNil())

				Eventually(func() map[string]string {
					// Check gateway back references
					err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, existingGateway)
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

				Eventually(func() error {
					// Check gateway back references
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway); err != nil {
						return err
					}
					cond := meta.FindStatusCondition(existingGateway.Status.Conditions, string(DNSPolicyAffected))
					if cond != nil {
						return fmt.Errorf("expected the condition %s to be gone", DNSPolicyAffected)
					}
					return nil
				}, time.Second*5, time.Second).Should(BeNil())
			})
		})

		Context("geo dnspolicy", func() {
			var dnsPolicy *v1alpha1.DNSPolicy

			BeforeEach(func() {
				dnsPolicy = testBuildDNSPolicyWithGeo("test-dns-policy", TestPlacedGatewayName, testNamespace)
				Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
				Eventually(func() error { //dns policy exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
						return err
					}
					return nil
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
			})

			AfterEach(func() {
				err := k8sClient.Delete(ctx, gateway)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should create a dns record", func() {
				createdDNSRecord := &v1alpha1.DNSRecord{}
				expectedEndpoints := []*v1alpha1.Endpoint{
					{
						DNSName: "2w705o.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							TestAttachedRouteAddressTwo,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "ie.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"2w705o.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "2w705o.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName: "ie.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"s07c46.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "s07c46.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
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
								Name:  "geo-code",
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
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "s07c46.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							TestAttachedRouteAddressOne,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "test.example.com",
						Targets: []string{
							"lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "",
						RecordTTL:     300,
					},
				}
				Eventually(func() error { // DNS record exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsRecordName, Namespace: dnsPolicy.Namespace}, createdDNSRecord); err != nil {
						return err
					}
					if len(createdDNSRecord.Spec.Endpoints) != len(expectedEndpoints) {
						return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", len(expectedEndpoints), len(createdDNSRecord.Spec.Endpoints))
					}
					return nil
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				Expect(createdDNSRecord.Spec.ManagedZoneRef.Name).To(Equal("example.com"))
				Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(7))
				Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
				Expect(expectedEndpoints).Should(ContainElements(createdDNSRecord.Spec.Endpoints))

			})

			It("should create a wildcard dns record", func() {
				wildcardDNSRecord := &v1alpha1.DNSRecord{}
				expectedEndpoints := []*v1alpha1.Endpoint{
					{
						DNSName: "*.example.com",
						Targets: []string{
							"lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "",
						RecordTTL:     300,
					},
					{
						DNSName: "2w705o.lb-" + lbHash + ".example.com",
						Targets: []string{
							TestAttachedRouteAddressTwo,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "ie.lb-" + lbHash + ".example.com",
						Targets: []string{
							"2w705o.lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "2w705o.lb-" + lbHash + ".example.com",
						RecordTTL:     60,
						Labels:        nil,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName: "ie.lb-" + lbHash + ".example.com",
						Targets: []string{
							"s07c46.lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "s07c46.lb-" + lbHash + ".example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName: "lb-" + lbHash + ".example.com",
						Targets: []string{
							"ie.lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "IE",
						RecordTTL:     300,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "geo-code",
								Value: "IE",
							},
						},
					},
					{
						DNSName: "lb-" + lbHash + ".example.com",
						Targets: []string{
							"ie.lb-" + lbHash + ".example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     300,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "s07c46.lb-" + lbHash + ".example.com",
						Targets: []string{
							TestAttachedRouteAddressOne,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
				}
				Eventually(func() error { // DNS record exists
					if err := k8sClient.Get(ctx, client.ObjectKey{Name: wildcardDNSRecordName, Namespace: dnsPolicy.Namespace}, wildcardDNSRecord); err != nil {
						return err
					}
					if len(wildcardDNSRecord.Spec.Endpoints) != len(expectedEndpoints) {
						return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", len(expectedEndpoints), len(wildcardDNSRecord.Spec.Endpoints))
					}
					return nil
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				Expect(wildcardDNSRecord.Spec.ManagedZoneRef.Name).To(Equal("example.com"))
				Expect(wildcardDNSRecord.Spec.Endpoints).To(HaveLen(7))
				Expect(wildcardDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
				Expect(expectedEndpoints).Should(ContainElements(wildcardDNSRecord.Spec.Endpoints))
			})
		})
	})

	Context("gateway not placed", func() {
		var gateway *gatewayv1beta1.Gateway
		var dnsPolicy *v1alpha1.DNSPolicy
		testGatewayName := "test-not-placed-gateway"

		BeforeEach(func() {
			gateway = testBuildGateway(testGatewayName, gatewayClass.Name, TestAttachedRouteName, testNamespace, "test-dns-policy")
			dnsPolicy = testBuildDNSPolicyWithHealthCheck("test-dns-policy", testGatewayName, testNamespace, nil)

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())

			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			Eventually(func() error { //gateway exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway); err != nil {
					return err
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			Eventually(func() error { //dns policy exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
		})

		AfterEach(func() {
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
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				if !meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
					return fmt.Errorf("expected DNSPolicy status condition to be %s", string(conditions.ConditionTypeReady))
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should set gateway back reference", func() {
			existingGateway := &gatewayv1beta1.Gateway{}
			policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
			refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
			policiesBackRefValue := string(refs)
			Eventually(func() error {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				if err != nil {
					return err
				}
				annotations := existingGateway.GetAnnotations()
				if annotations == nil {
					return fmt.Errorf("existingGateway annotations should not be nil")
				}
				if _, ok := annotations[DNSPolicyBackRefAnnotation]; !ok {
					return fmt.Errorf("existingGateway annotations do not have annotation %s", DNSPolicyBackRefAnnotation)
				}
				if annotations[DNSPolicyBackRefAnnotation] != policyBackRefValue {
					return fmt.Errorf("existingGateway annotations[%s] does not have expected value", DNSPolicyBackRefAnnotation)
				}
				return nil
			}, time.Second*5, time.Second).Should(BeNil())
			Eventually(func() error {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				if err != nil {
					return err
				}
				annotations := existingGateway.GetAnnotations()
				if annotations == nil {
					return fmt.Errorf("existingGateway annotations should not be nil")
				}
				if _, ok := annotations[DNSPoliciesBackRefAnnotation]; !ok {
					return fmt.Errorf("existingGateway annotations do not have annotation %s", DNSPoliciesBackRefAnnotation)
				}
				if annotations[DNSPoliciesBackRefAnnotation] != policiesBackRefValue {
					return fmt.Errorf("existingGateway annotations[%s] does not have expected value", DNSPoliciesBackRefAnnotation)
				}
				return nil
			}, time.Second*5, time.Second).Should(BeNil())
		})
	})

	Context("probes status impact DNS records", func() {
		var gateway *gatewayv1beta1.Gateway
		var dnsRecordName, lbHash string
		var dnsPolicy *v1alpha1.DNSPolicy
		var unhealthy bool

		BeforeEach(func() {
			gateway = testBuildGateway(TestPlacedGatewayName, gatewayClass.Name, TestAttachedRouteName, testNamespace, "test-dns-policy")
			dnsRecordName = fmt.Sprintf("%s-%s", TestPlacedGatewayName, TestAttachedRouteName)
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() error { //gateway exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway); err != nil {
					return err
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			threshold := 4
			dnsPolicy = testBuildDNSPolicyWithHealthCheck("test-dns-policy", TestPlacedGatewayName, testNamespace, &threshold)
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
			Eventually(func() error { //dns policy exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
		})

		AfterEach(func() {
			//clean up gateway
			gatewayList := &gatewayv1beta1.GatewayList{}
			Expect(k8sClient.List(ctx, gatewayList)).To(BeNil())
			for _, gw := range gatewayList.Items {
				k8sClient.Delete(ctx, &gw)
			}
		})

		It("should create a dns record", func() {
			createdDNSRecord := &v1alpha1.DNSRecord{}
			Eventually(func() error { // DNS record exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsRecordName, Namespace: testNamespace}, createdDNSRecord); err != nil {
					return err
				}
				if len(createdDNSRecord.Spec.Endpoints) != 6 {
					return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", 6, len(createdDNSRecord.Spec.Endpoints))
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
		})
		It("should have probes that are healthy", func() {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			Expect(err).NotTo(HaveOccurred())
			patch := client.MergeFrom(gateway.DeepCopy())
			addressType := mgcgateway.MultiClusterIPAddressType
			gateway.Status.Addresses = []gatewayv1beta1.GatewayAddress{
				{
					Type:  &addressType,
					Value: fmt.Sprintf("%s/%s", "kind-mgc-control-plane", TestAttachedRouteAddressOne),
				},
				{
					Type:  &addressType,
					Value: fmt.Sprintf("%s/%s", "kind-mgc-control-plane", TestAttachedRouteAddressTwo),
				},
			}
			Expect(k8sClient.Status().Patch(ctx, gateway, patch)).To(BeNil())

			probeList := &v1alpha1.DNSHealthCheckProbeList{}
			Eventually(func() error {
				Expect(k8sClient.List(ctx, probeList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
				if len(probeList.Items) != 2 {
					return fmt.Errorf("expected %v probes, got %v", 2, len(probeList.Items))
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
			Expect(len(probeList.Items)).To(Equal(2))
		})

		Context("all unhealthy probes", func() {
			It("should publish all dns records endpoints", func() {
				lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))

				expectedEndpoints := []*v1alpha1.Endpoint{
					{
						DNSName: "2w705o.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							TestAttachedRouteAddressTwo,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "s07c46.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							TestAttachedRouteAddressOne,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "default.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"2w705o.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "2w705o.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName: "default.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"s07c46.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "s07c46.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						Labels:        nil,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
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
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "test.example.com",
						Targets: []string{
							"lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "",
						RecordTTL:     300,
					},
				}

				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				Expect(err).NotTo(HaveOccurred())
				patch := client.MergeFrom(gateway.DeepCopy())
				addressType := mgcgateway.MultiClusterIPAddressType
				gateway.Status.Addresses = []gatewayv1beta1.GatewayAddress{
					{
						Type:  &addressType,
						Value: fmt.Sprintf("%s/%s", "kind-mgc-control-plane", TestAttachedRouteAddressOne),
					},
					{
						Type:  &addressType,
						Value: fmt.Sprintf("%s/%s", "kind-mgc-control-plane", TestAttachedRouteAddressTwo),
					},
				}
				Expect(k8sClient.Status().Patch(ctx, gateway, patch)).To(BeNil())

				probeList := &v1alpha1.DNSHealthCheckProbeList{}
				Eventually(func() error {
					Expect(k8sClient.List(ctx, probeList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
					if len(probeList.Items) != 2 {
						return fmt.Errorf("expected %v probes, got %v", 2, len(probeList.Items))
					}
					return nil
				}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())

				for _, probe := range probeList.Items {
					Eventually(func() error {
						if probe.Name == fmt.Sprintf("%s-test-dns-policy-%s", TestAttachedRouteAddressTwo, TestAttachedRouteName) ||
							probe.Name == fmt.Sprintf("%s-test-dns-policy-%s", TestAttachedRouteAddressOne, TestAttachedRouteName) {
							getProbe := &v1alpha1.DNSHealthCheckProbe{}
							if err = k8sClient.Get(ctx, client.ObjectKey{Name: probe.Name, Namespace: probe.Namespace}, getProbe); err != nil {
								return err
							}
							patch := client.MergeFrom(getProbe.DeepCopy())
							unhealthy = false
							getProbe.Status = v1alpha1.DNSHealthCheckProbeStatus{
								LastCheckedAt:       metav1.NewTime(time.Now()),
								ConsecutiveFailures: *getProbe.Spec.FailureThreshold + 1,
								Healthy:             &unhealthy,
							}
							if err = k8sClient.Status().Patch(ctx, getProbe, patch); err != nil {
								return err
							}
						}
						return nil
					}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				}
				createdDNSRecord := &v1alpha1.DNSRecord{}
				Eventually(func() error {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsRecordName, Namespace: testNamespace}, createdDNSRecord)
					if err != nil && k8serrors.IsNotFound(err) {
						return err
					}
					if len(createdDNSRecord.Spec.Endpoints) != len(expectedEndpoints) {
						return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", len(expectedEndpoints), len(createdDNSRecord.Spec.Endpoints))
					}
					return nil
				}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())
				Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(6))
				Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
				Expect(expectedEndpoints).Should(ContainElements(createdDNSRecord.Spec.Endpoints))

			})
		})
		Context("some unhealthy endpoints", func() {
			It("should publish expected endpoints", func() {
				lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))

				expectedEndpoints := []*v1alpha1.Endpoint{
					{
						DNSName: "2w705o.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							TestAttachedRouteAddressTwo,
						},
						RecordType:    "A",
						SetIdentifier: "",
						RecordTTL:     60,
					},
					{
						DNSName: "default.lb-" + lbHash + ".test.example.com",
						Targets: []string{
							"2w705o.lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "2w705o.lb-" + lbHash + ".test.example.com",
						RecordTTL:     60,
						ProviderSpecific: v1alpha1.ProviderSpecific{
							{
								Name:  "weight",
								Value: "120",
							},
						},
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
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName: "test.example.com",
						Targets: []string{
							"lb-" + lbHash + ".test.example.com",
						},
						RecordType:    "CNAME",
						SetIdentifier: "",
						RecordTTL:     300,
					},
				}

				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				Expect(err).NotTo(HaveOccurred())
				patch := client.MergeFrom(gateway.DeepCopy())
				addressType := mgcgateway.MultiClusterIPAddressType
				gateway.Status.Addresses = []gatewayv1beta1.GatewayAddress{
					{
						Type:  &addressType,
						Value: fmt.Sprintf("%s/%s", "kind-mgc-control-plane", TestAttachedRouteAddressOne),
					},
					{
						Type:  &addressType,
						Value: fmt.Sprintf("%s/%s", "kind-mgc-control-plane", TestAttachedRouteAddressTwo),
					},
				}
				Expect(k8sClient.Status().Patch(ctx, gateway, patch)).To(BeNil())

				probeList := &v1alpha1.DNSHealthCheckProbeList{}
				Eventually(func() error {
					Expect(k8sClient.List(ctx, probeList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
					if len(probeList.Items) != 2 {
						return fmt.Errorf("expected %v probes, got %v", 2, len(probeList.Items))
					}
					return nil
				}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())
				Expect(len(probeList.Items)).To(Equal(2))

				Eventually(func() error {
					getProbe := &v1alpha1.DNSHealthCheckProbe{}
					if err = k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-test-dns-policy-%s", TestAttachedRouteAddressOne, TestAttachedRouteName), Namespace: testNamespace}, getProbe); err != nil {
						return err
					}
					patch := client.MergeFrom(getProbe.DeepCopy())
					unhealthy = false
					getProbe.Status = v1alpha1.DNSHealthCheckProbeStatus{
						LastCheckedAt:       metav1.NewTime(time.Now()),
						ConsecutiveFailures: *getProbe.Spec.FailureThreshold + 1,
						Healthy:             &unhealthy,
					}
					if err = k8sClient.Status().Patch(ctx, getProbe, patch); err != nil {
						return err
					}
					return nil
				}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())

				// after that verify that in time the endpoints are 5 in the dnsrecord
				createdDNSRecord := &v1alpha1.DNSRecord{}
				Eventually(func() error {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsRecordName, Namespace: testNamespace}, createdDNSRecord)
					if err != nil && k8serrors.IsNotFound(err) {
						return err
					}
					if len(createdDNSRecord.Spec.Endpoints) != len(expectedEndpoints) {
						return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", len(expectedEndpoints), len(createdDNSRecord.Spec.Endpoints))
					}
					return nil
				}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(4))
				Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
				Expect(expectedEndpoints).Should(ContainElements(createdDNSRecord.Spec.Endpoints))
			})
		})
	})
})
