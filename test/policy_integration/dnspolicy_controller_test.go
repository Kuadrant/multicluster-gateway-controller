//go:build integration

package policy_integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	v1 "open-cluster-management.io/api/cluster/v1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnspolicy"
	mgcgateway "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("DNSPolicy", Ordered, func() {

	var gatewayClass *gatewayv1beta1.GatewayClass
	var managedZone *v1alpha1.ManagedZone
	var testNamespace string
	var dnsPolicyBuilder *testutil.DNSPolicyBuilder
	var gateway *gatewayv1beta1.Gateway
	var dnsPolicy *v1alpha1.DNSPolicy

	BeforeAll(func() {
		logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
		logger.WithName("dnspolicy_controller_test")
		logf.SetLogger(logger)

		gatewayClass = testutil.NewTestGatewayClass("foo", "default", "kuadrant.io/bar")
		Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
		Eventually(func() error { // gateway class exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, gatewayClass)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		CreateNamespace(&testNamespace)
		managedZone = testutil.NewManagedZoneBuilder("mz-example-com", testNamespace, "example.com").ManagedZone
		Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())
		Eventually(func() error { // managed zone exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: managedZone.Name, Namespace: managedZone.Namespace}, managedZone)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		dnsPolicyBuilder = testutil.NewDNSPolicyBuilder("test-dns-policy", testNamespace)
	})

	AfterEach(func() {
		if gateway != nil {
			err := k8sClient.Delete(ctx, gateway)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}

		if dnsPolicy != nil {
			err := k8sClient.Delete(ctx, dnsPolicy)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

		}
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

	Context("invalid target", func() {

		BeforeEach(func() {
			dnsPolicy = dnsPolicyBuilder.
				WithTargetGateway("test-gateway").
				WithRoutingStrategy(v1alpha1.SimpleRoutingStrategy).
				DNSPolicy
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
			Eventually(func() error { //dns policy exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		})

		It("should have ready condition with status false and correct reason", func() {
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				readyCond := meta.FindStatusCondition(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady))
				if readyCond == nil {
					return fmt.Errorf("expected dnsPolicy to have %s condition, got none",
						string(conditions.ConditionTypeReady))
				}
				if readyCond.Status != metav1.ConditionFalse {
					return fmt.Errorf("expected dnsPolicy %s condition to have status %s, got %s",
						string(conditions.ConditionTypeReady), metav1.ConditionFalse, readyCond.Status)
				}
				if readyCond.Reason != string(conditions.PolicyReasonTargetNotFound) {
					return fmt.Errorf("expected dnsPolicy %s condition to have reason %s, got %s",
						string(conditions.ConditionTypeReady), string(conditions.PolicyReasonTargetNotFound), readyCond.Reason)
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should have ready condition with status true", func() {
			By("creating a valid Gateway")

			gateway = testutil.NewGatewayBuilder("test-gateway", gatewayClass.Name, testNamespace).
				WithHTTPListener("test-listener", "test.example.com").Gateway
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				if !meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
					return fmt.Errorf("expected dnsPolicy %s condition to have status %s ", string(conditions.ConditionTypeReady), metav1.ConditionTrue)
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should not have any health check records created", func() {
			// create a health check with the labels for the dnspolicy and the gateway name and namespace that would be expected in a valid target scenario
			// this one should get deleted if the gateway is invalid policy ref
			probe := &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-%s", TestIPAddressTwo, TestGatewayName, TestHostOne),
					Namespace: testNamespace,
					Labels: map[string]string{
						DNSPolicyBackRefAnnotation:                              "test-dns-policy",
						fmt.Sprintf("%s-namespace", DNSPolicyBackRefAnnotation): testNamespace,
						LabelGatewayNSRef:                                       testNamespace,
						LabelGatewayReference:                                   "test-gateway",
					},
				},
			}
			Expect(k8sClient.Create(ctx, probe)).To(BeNil())

			Eventually(func() error { // probe should be present
				getCreatedProbe := &v1alpha1.DNSHealthCheckProbe{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: probe.Name, Namespace: probe.Namespace}, getCreatedProbe)
				return err
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			Eventually(func() bool { // probe should be removed
				getDeletedProbe := &v1alpha1.DNSHealthCheckProbe{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: probe.Name, Namespace: probe.Namespace}, getDeletedProbe)
				if err != nil {
					if k8serrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

	})

	Context("valid target with no gateway status", func() {
		testGatewayName := "test-no-gateway-status"

		BeforeEach(func() {
			gateway = testutil.NewGatewayBuilder(testGatewayName, gatewayClass.Name, testNamespace).
				WithHTTPListener(TestListenerNameOne, TestHostOne).
				WithHTTPListener(TestListenerNameWildcard, TestHostWildcard).
				Gateway
			dnsPolicy = dnsPolicyBuilder.
				WithTargetGateway(testGatewayName).
				WithRoutingStrategy(v1alpha1.SimpleRoutingStrategy).
				DNSPolicy

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())

			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			Eventually(func() error { //dns policy exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
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

	Context("multi cluster gateway status", func() {
		var lbHash, recordName, wildcardRecordName string

		BeforeEach(func() {
			gateway = testutil.NewGatewayBuilder(TestGatewayName, gatewayClass.Name, testNamespace).
				WithHTTPListener(TestListenerNameOne, TestHostOne).
				WithHTTPListener(TestListenerNameWildcard, TestHostWildcard).
				Gateway
			dnsPolicyBuilder.WithTargetGateway(TestGatewayName)
			lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))
			recordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameOne)
			wildcardRecordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameWildcard)
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			Eventually(func() error {
				if err := k8sClient.Create(ctx, &v1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: TestClusterNameOne,
					},
				}); err != nil && !k8serrors.IsAlreadyExists(err) {
					return err
				}
				if err := k8sClient.Create(ctx, &v1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: TestClusterNameTwo,
					},
				}); err != nil && !k8serrors.IsAlreadyExists(err) {
					return err
				}
				gateway.Status.Addresses = []gatewayv1beta1.GatewayAddress{
					{
						Type:  testutil.Pointer(mgcgateway.MultiClusterIPAddressType),
						Value: TestClusterNameOne + "/" + TestIPAddressOne,
					},
					{
						Type:  testutil.Pointer(mgcgateway.MultiClusterIPAddressType),
						Value: TestClusterNameTwo + "/" + TestIPAddressTwo,
					},
				}
				gateway.Status.Listeners = []gatewayv1beta1.ListenerStatus{
					{
						Name:           TestClusterNameOne + "." + TestListenerNameOne,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
					{
						Name:           TestClusterNameTwo + "." + TestListenerNameOne,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
					{
						Name:           TestClusterNameOne + "." + TestListenerNameWildcard,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
					{
						Name:           TestClusterNameTwo + "." + TestListenerNameWildcard,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
				}
				return k8sClient.Status().Update(ctx, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			dnsRecordList := &v1alpha1.DNSRecordList{}
			err := k8sClient.List(ctx, dnsRecordList)
			Expect(err).ToNot(HaveOccurred())

			for _, record := range dnsRecordList.Items {
				err := k8sClient.Delete(ctx, &record)
				Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
			}
		})

		Context("simple routing strategy", func() {

			BeforeEach(func() {
				dnsPolicyBuilder.WithRoutingStrategy(v1alpha1.SimpleRoutingStrategy)
				dnsPolicy = dnsPolicyBuilder.DNSPolicy
				Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
				Eventually(func() error { //dns policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create dns records", func() {
				Eventually(func(g Gomega, ctx context.Context) {
					recordList := &v1alpha1.DNSRecordList{}
					err := k8sClient.List(ctx, recordList, &client.ListOptions{Namespace: testNamespace})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(recordList.Items).To(HaveLen(2))
					g.Expect(recordList.Items).To(
						ContainElements(
							MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": HaveField("Name", recordName),
								"Spec": MatchFields(IgnoreExtras, Fields{
									"ManagedZoneRef": HaveField("Name", "mz-example-com"),
									"Endpoints": ConsistOf(
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal(TestHostOne),
											"Targets":       ContainElements(TestIPAddressOne, TestIPAddressTwo),
											"RecordType":    Equal("A"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(60)),
										})),
									),
								}),
							}),
							MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": HaveField("Name", wildcardRecordName),
								"Spec": MatchFields(IgnoreExtras, Fields{
									"ManagedZoneRef": HaveField("Name", "mz-example-com"),
									"Endpoints": ConsistOf(
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal(TestHostWildcard),
											"Targets":       ContainElements(TestIPAddressOne, TestIPAddressTwo),
											"RecordType":    Equal("A"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(60)),
										})),
									),
								}),
							}),
						))
				}, TestTimeoutMedium, TestRetryIntervalMedium, ctx).Should(Succeed())
			})

		})

		Context("loadbalanced routing strategy", func() {

			BeforeEach(func() {
				dnsPolicyBuilder.WithRoutingStrategy(v1alpha1.LoadBalancedRoutingStrategy)
			})

			Context("weighted", func() {

				BeforeEach(func() {
					dnsPolicyBuilder.WithLoadBalancingWeightedFor(120, nil)
					dnsPolicy = dnsPolicyBuilder.DNSPolicy
					Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
					Eventually(func() error { //dns policy exists
						return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
					}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				})

				It("should create dns records", func() {
					Eventually(func(g Gomega, ctx context.Context) {
						recordList := &v1alpha1.DNSRecordList{}
						err := k8sClient.List(ctx, recordList, &client.ListOptions{Namespace: testNamespace})
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(recordList.Items).To(HaveLen(2))
						g.Expect(recordList.Items).To(
							ContainElements(
								MatchFields(IgnoreExtras, Fields{
									"ObjectMeta": HaveField("Name", recordName),
									"Spec": MatchFields(IgnoreExtras, Fields{
										"ManagedZoneRef": HaveField("Name", "mz-example-com"),
										"Endpoints": ConsistOf(
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("2w705o.lb-" + lbHash + ".test.example.com"),
												"Targets":       ConsistOf(TestIPAddressTwo),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("default.lb-" + lbHash + ".test.example.com"),
												"Targets":          ConsistOf("2w705o.lb-" + lbHash + ".test.example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("2w705o.lb-" + lbHash + ".test.example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("default.lb-" + lbHash + ".test.example.com"),
												"Targets":          ConsistOf("s07c46.lb-" + lbHash + ".test.example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("s07c46.lb-" + lbHash + ".test.example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("s07c46.lb-" + lbHash + ".test.example.com"),
												"Targets":       ConsistOf(TestIPAddressOne),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("lb-" + lbHash + ".test.example.com"),
												"Targets":          ConsistOf("default.lb-" + lbHash + ".test.example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("default"),
												"RecordTTL":        Equal(v1alpha1.TTL(300)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal(TestHostOne),
												"Targets":       ConsistOf("lb-" + lbHash + ".test.example.com"),
												"RecordType":    Equal("CNAME"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(300)),
											})),
										),
									}),
								}),
								MatchFields(IgnoreExtras, Fields{
									"ObjectMeta": HaveField("Name", wildcardRecordName),
									"Spec": MatchFields(IgnoreExtras, Fields{
										"ManagedZoneRef": HaveField("Name", "mz-example-com"),
										"Endpoints": ConsistOf(
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("2w705o.lb-" + lbHash + ".example.com"),
												"Targets":       ConsistOf(TestIPAddressTwo),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("default.lb-" + lbHash + ".example.com"),
												"Targets":          ConsistOf("2w705o.lb-" + lbHash + ".example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("2w705o.lb-" + lbHash + ".example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("default.lb-" + lbHash + ".example.com"),
												"Targets":          ConsistOf("s07c46.lb-" + lbHash + ".example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("s07c46.lb-" + lbHash + ".example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("s07c46.lb-" + lbHash + ".example.com"),
												"Targets":       ConsistOf(TestIPAddressOne),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("lb-" + lbHash + ".example.com"),
												"Targets":          ConsistOf("default.lb-" + lbHash + ".example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("default"),
												"RecordTTL":        Equal(v1alpha1.TTL(300)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal(TestHostWildcard),
												"Targets":       ConsistOf("lb-" + lbHash + ".example.com"),
												"RecordType":    Equal("CNAME"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(300)),
											})),
										),
									}),
								}),
							))
					}, TestTimeoutMedium, TestRetryIntervalMedium, ctx).Should(Succeed())
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
							if existing.Name == TestListenerNameWildcard {
								newListeners = append(newListeners, existing)
							}
						}

						patch := client.MergeFrom(existingGateway.DeepCopy())
						existingGateway.Spec.Listeners = newListeners
						rec := &v1alpha1.DNSRecord{}
						if err := k8sClient.Patch(ctx, existingGateway, patch); err != nil {
							return err
						}
						//dns record should be removed for non wildcard
						if err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, rec); err != nil && !k8serrors.IsNotFound(err) {
							return err
						}
						return k8sClient.Get(ctx, client.ObjectKey{Name: wildcardRecordName, Namespace: testNamespace}, rec)
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

				It("should remove dns record reference on policy deletion even if gateway is removed", func() {
					createdDNSRecord := &v1alpha1.DNSRecord{}
					Eventually(func() error { // DNS record exists
						if err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord); err != nil {
							return err
						}
						return nil
					}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

					err := k8sClient.Delete(ctx, gateway)
					Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

					err = k8sClient.Delete(ctx, dnsPolicy)
					Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

					Eventually(func() error { // DNS record removed
						if err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord); err != nil {
							if k8serrors.IsNotFound(err) {
								return nil
							}
							return err
						}
						return errors.New("found dnsrecord when it should be deleted")
					}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				})

			})

			Context("geo+weighted", func() {

				BeforeEach(func() {
					dnsPolicyBuilder.
						WithLoadBalancingWeightedFor(120, nil).
						WithLoadBalancingGeoFor("IE")
					dnsPolicy = dnsPolicyBuilder.DNSPolicy
					Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
					Eventually(func() error { //dns policy exists
						if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
							return err
						}
						return nil
					}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
				})

				It("should create dns records", func() {
					Eventually(func(g Gomega, ctx context.Context) {
						recordList := &v1alpha1.DNSRecordList{}
						err := k8sClient.List(ctx, recordList, &client.ListOptions{Namespace: testNamespace})
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(recordList.Items).To(HaveLen(2))
						g.Expect(recordList.Items).To(
							ContainElements(
								MatchFields(IgnoreExtras, Fields{
									"ObjectMeta": HaveField("Name", recordName),
									"Spec": MatchFields(IgnoreExtras, Fields{
										"ManagedZoneRef": HaveField("Name", "mz-example-com"),
										"Endpoints": ConsistOf(
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("2w705o.lb-" + lbHash + ".test.example.com"),
												"Targets":       ConsistOf(TestIPAddressTwo),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("ie.lb-" + lbHash + ".test.example.com"),
												"Targets":          ConsistOf("2w705o.lb-" + lbHash + ".test.example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("2w705o.lb-" + lbHash + ".test.example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("ie.lb-" + lbHash + ".test.example.com"),
												"Targets":          ConsistOf("s07c46.lb-" + lbHash + ".test.example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("s07c46.lb-" + lbHash + ".test.example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("s07c46.lb-" + lbHash + ".test.example.com"),
												"Targets":       ConsistOf(TestIPAddressOne),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("lb-" + lbHash + ".test.example.com"),
												"Targets":          ConsistOf("ie.lb-" + lbHash + ".test.example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("IE"),
												"RecordTTL":        Equal(v1alpha1.TTL(300)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "IE"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("lb-" + lbHash + ".test.example.com"),
												"Targets":          ConsistOf("ie.lb-" + lbHash + ".test.example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("default"),
												"RecordTTL":        Equal(v1alpha1.TTL(300)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal(TestHostOne),
												"Targets":       ConsistOf("lb-" + lbHash + ".test.example.com"),
												"RecordType":    Equal("CNAME"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(300)),
											})),
										),
									}),
								}),
								MatchFields(IgnoreExtras, Fields{
									"ObjectMeta": HaveField("Name", wildcardRecordName),
									"Spec": MatchFields(IgnoreExtras, Fields{
										"ManagedZoneRef": HaveField("Name", "mz-example-com"),
										"Endpoints": ConsistOf(
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("2w705o.lb-" + lbHash + ".example.com"),
												"Targets":       ConsistOf(TestIPAddressTwo),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("ie.lb-" + lbHash + ".example.com"),
												"Targets":          ConsistOf("2w705o.lb-" + lbHash + ".example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("2w705o.lb-" + lbHash + ".example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("ie.lb-" + lbHash + ".example.com"),
												"Targets":          ConsistOf("s07c46.lb-" + lbHash + ".example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("s07c46.lb-" + lbHash + ".example.com"),
												"RecordTTL":        Equal(v1alpha1.TTL(60)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal("s07c46.lb-" + lbHash + ".example.com"),
												"Targets":       ConsistOf(TestIPAddressOne),
												"RecordType":    Equal("A"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(60)),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("lb-" + lbHash + ".example.com"),
												"Targets":          ConsistOf("ie.lb-" + lbHash + ".example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("IE"),
												"RecordTTL":        Equal(v1alpha1.TTL(300)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "IE"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":          Equal("lb-" + lbHash + ".example.com"),
												"Targets":          ConsistOf("ie.lb-" + lbHash + ".example.com"),
												"RecordType":       Equal("CNAME"),
												"SetIdentifier":    Equal("default"),
												"RecordTTL":        Equal(v1alpha1.TTL(300)),
												"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
											})),
											PointTo(MatchFields(IgnoreExtras, Fields{
												"DNSName":       Equal(TestHostWildcard),
												"Targets":       ConsistOf("lb-" + lbHash + ".example.com"),
												"RecordType":    Equal("CNAME"),
												"SetIdentifier": Equal(""),
												"RecordTTL":     Equal(v1alpha1.TTL(300)),
											})),
										),
									}),
								}),
							))
					}, TestTimeoutMedium, TestRetryIntervalMedium, ctx).Should(Succeed())
				})

			})

			Context("with health checks", func() {
				var unhealthy bool

				BeforeEach(func() {
					dnsPolicyBuilder.
						WithLoadBalancingWeightedFor(120, nil).
						WithHealthCheckFor("/", nil, v1alpha1.HttpProtocol, testutil.Pointer(4))
					dnsPolicy = dnsPolicyBuilder.DNSPolicy
					Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
					Eventually(func() error { //dns policy exists
						return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
					}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
				})

				It("should create a dns record", func() {
					createdDNSRecord := &v1alpha1.DNSRecord{}
					Eventually(func() error { // DNS record exists
						if err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord); err != nil {
							return err
						}
						if len(createdDNSRecord.Spec.Endpoints) != 6 {
							return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", 6, len(createdDNSRecord.Spec.Endpoints))
						}
						return nil
					}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
					Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(6))
				})
				It("should have probes that are healthy", func() {
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
									TestIPAddressTwo,
								},
								RecordType:    "A",
								SetIdentifier: "",
								RecordTTL:     60,
							},
							{
								DNSName: "s07c46.lb-" + lbHash + ".test.example.com",
								Targets: []string{
									TestIPAddressOne,
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
								if probe.Name == fmt.Sprintf("%s-%s-%s", TestIPAddressTwo, TestGatewayName, TestHostOne) ||
									probe.Name == fmt.Sprintf("%s-%s-%s", TestIPAddressOne, TestGatewayName, TestHostOne) {
									getProbe := &v1alpha1.DNSHealthCheckProbe{}
									if err := k8sClient.Get(ctx, client.ObjectKey{Name: probe.Name, Namespace: probe.Namespace}, getProbe); err != nil {
										return err
									}
									patch := client.MergeFrom(getProbe.DeepCopy())
									unhealthy = false
									getProbe.Status = v1alpha1.DNSHealthCheckProbeStatus{
										LastCheckedAt:       metav1.NewTime(time.Now()),
										ConsecutiveFailures: *getProbe.Spec.FailureThreshold + 1,
										Healthy:             &unhealthy,
									}
									if err := k8sClient.Status().Patch(ctx, getProbe, patch); err != nil {
										return err
									}
								}
								return nil
							}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
						}
						createdDNSRecord := &v1alpha1.DNSRecord{}
						Eventually(func() error {

							err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord)
							if err != nil && k8serrors.IsNotFound(err) {
								return err
							}
							if len(createdDNSRecord.Spec.Endpoints) != len(expectedEndpoints) {
								return fmt.Errorf("expected %v endpoints in DNSRecord, got %v", len(expectedEndpoints), len(createdDNSRecord.Spec.Endpoints))
							}
							return nil
						}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())
						Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(len(expectedEndpoints)))
						Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
						Expect(expectedEndpoints).Should(ContainElements(createdDNSRecord.Spec.Endpoints))

					})
				})
				Context("some unhealthy probes", func() {
					It("should publish expected endpoints", func() {
						lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))

						expectedEndpoints := []*v1alpha1.Endpoint{
							{
								DNSName: "2w705o.lb-" + lbHash + ".test.example.com",
								Targets: []string{
									TestIPAddressTwo,
								},
								RecordType:    "A",
								SetIdentifier: "",
								RecordTTL:     60,
							},
							{
								DNSName: "s07c46.lb-" + lbHash + ".test.example.com",
								Targets: []string{
									TestIPAddressOne,
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

						probeList := &v1alpha1.DNSHealthCheckProbeList{}
						Eventually(func() error {
							Expect(k8sClient.List(ctx, probeList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
							if len(probeList.Items) != 2 {
								return fmt.Errorf("expected %v probes, got %v", 2, len(probeList.Items))
							}
							return nil
						}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())
						Expect(probeList.Items).To(HaveLen(2))

						Eventually(func() error {
							getProbe := &v1alpha1.DNSHealthCheckProbe{}
							if err := k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-%s-%s", TestIPAddressOne, TestGatewayName, TestListenerNameOne), Namespace: testNamespace}, getProbe); err != nil {
								return err
							}
							patch := client.MergeFrom(getProbe.DeepCopy())
							unhealthy = false
							getProbe.Status = v1alpha1.DNSHealthCheckProbeStatus{
								LastCheckedAt:       metav1.NewTime(time.Now()),
								ConsecutiveFailures: *getProbe.Spec.FailureThreshold + 1,
								Healthy:             &unhealthy,
							}
							if err := k8sClient.Status().Patch(ctx, getProbe, patch); err != nil {
								return err
							}
							return nil
						}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())

						// after that verify that in time the endpoints are 5 in the dnsrecord
						createdDNSRecord := &v1alpha1.DNSRecord{}
						Eventually(func() error {
							err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord)
							if err != nil && k8serrors.IsNotFound(err) {
								return err
							}
							return nil
						}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
						Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(len(expectedEndpoints)))
						Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
						Expect(expectedEndpoints).Should(ContainElements(createdDNSRecord.Spec.Endpoints))
					})
				})
				Context("some unhealthy endpoints for other listener", func() {
					It("should publish expected endpoints", func() {
						lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))

						expectedEndpoints := []*v1alpha1.Endpoint{
							{
								DNSName: "2w705o.lb-" + lbHash + ".test.example.com",
								Targets: []string{
									TestIPAddressTwo,
								},
								RecordType:    "A",
								SetIdentifier: "",
								RecordTTL:     60,
							},
							{
								DNSName: "s07c46.lb-" + lbHash + ".test.example.com",
								Targets: []string{
									TestIPAddressOne,
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
						Expect(gateway.Spec.Listeners).NotTo(BeNil())
						// add another listener, should result in 4 probes
						typedHostname := gatewayv1beta1.Hostname(TestHostTwo)
						otherListener := gatewayv1beta1.Listener{
							Name:     gatewayv1beta1.SectionName(TestListenerNameTwo),
							Hostname: &typedHostname,
							Port:     gatewayv1beta1.PortNumber(80),
							Protocol: gatewayv1beta1.HTTPProtocolType,
						}

						patch := client.MergeFrom(gateway.DeepCopy())
						gateway.Spec.Listeners = append(gateway.Spec.Listeners, otherListener)
						Expect(k8sClient.Patch(ctx, gateway, patch)).To(BeNil())

						probeList := &v1alpha1.DNSHealthCheckProbeList{}
						Eventually(func() error {
							Expect(k8sClient.List(ctx, probeList, &client.ListOptions{Namespace: testNamespace})).To(BeNil())
							if len(probeList.Items) != 4 {
								return fmt.Errorf("expected %v probes, got %v", 4, len(probeList.Items))
							}
							return nil
						}, TestTimeoutLong, TestRetryIntervalMedium).Should(BeNil())
						Expect(len(probeList.Items)).To(Equal(4))

						//
						Eventually(func() error {
							getProbe := &v1alpha1.DNSHealthCheckProbe{}
							if err = k8sClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s-%s-%s", TestIPAddressOne, TestGatewayName, TestListenerNameTwo), Namespace: testNamespace}, getProbe); err != nil {
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
							err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord)
							if err != nil && k8serrors.IsNotFound(err) {
								return err
							}
							return nil
						}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
						Expect(createdDNSRecord.Spec.Endpoints).To(HaveLen(len(expectedEndpoints)))
						Expect(createdDNSRecord.Spec.Endpoints).Should(ContainElements(expectedEndpoints))
						Expect(expectedEndpoints).Should(ContainElements(createdDNSRecord.Spec.Endpoints))
					})
				})
			})

		})

	})

	Context("single cluster gateway status", func() {
		var lbHash, recordName, wildcardRecordName string

		BeforeEach(func() {
			gateway = testutil.NewGatewayBuilder(TestGatewayName, gatewayClass.Name, testNamespace).
				WithHTTPListener(TestListenerNameOne, TestHostOne).
				WithHTTPListener(TestListenerNameWildcard, TestHostWildcard).
				Gateway
			dnsPolicyBuilder.WithTargetGateway(TestGatewayName)
			lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))
			recordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameOne)
			wildcardRecordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameWildcard)
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			Eventually(func() error {
				gateway.Status.Addresses = []gatewayv1beta1.GatewayAddress{
					{
						Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
						Value: TestIPAddressOne,
					},
					{
						Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
						Value: TestIPAddressTwo,
					},
				}
				gateway.Status.Listeners = []gatewayv1beta1.ListenerStatus{
					{
						Name:           TestListenerNameOne,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
					{
						Name:           TestListenerNameWildcard,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
				}
				return k8sClient.Status().Update(ctx, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			dnsRecordList := &v1alpha1.DNSRecordList{}
			err := k8sClient.List(ctx, dnsRecordList)
			Expect(err).ToNot(HaveOccurred())

			for _, record := range dnsRecordList.Items {
				err := k8sClient.Delete(ctx, &record)
				Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
			}
		})

		Context("simple routing strategy", func() {

			BeforeEach(func() {
				dnsPolicyBuilder.WithRoutingStrategy(v1alpha1.SimpleRoutingStrategy)
				dnsPolicy = dnsPolicyBuilder.DNSPolicy
				Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
				Eventually(func() error { //dns policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create dns records", func() {
				Eventually(func(g Gomega, ctx context.Context) {
					recordList := &v1alpha1.DNSRecordList{}
					err := k8sClient.List(ctx, recordList, &client.ListOptions{Namespace: testNamespace})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(recordList.Items).To(HaveLen(2))
					g.Expect(recordList.Items).To(
						ContainElements(
							MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": HaveField("Name", recordName),
								"Spec": MatchFields(IgnoreExtras, Fields{
									"ManagedZoneRef": HaveField("Name", "mz-example-com"),
									"Endpoints": ConsistOf(
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal(TestHostOne),
											"Targets":       ContainElements(TestIPAddressOne, TestIPAddressTwo),
											"RecordType":    Equal("A"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(60)),
										})),
									),
								}),
							}),
							MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": HaveField("Name", wildcardRecordName),
								"Spec": MatchFields(IgnoreExtras, Fields{
									"ManagedZoneRef": HaveField("Name", "mz-example-com"),
									"Endpoints": ConsistOf(
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal(TestHostWildcard),
											"Targets":       ContainElements(TestIPAddressOne, TestIPAddressTwo),
											"RecordType":    Equal("A"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(60)),
										})),
									),
								}),
							}),
						))
				}, TestTimeoutMedium, TestRetryIntervalMedium, ctx).Should(Succeed())
			})

		})

		Context("loadbalanced routing strategy", func() {

			BeforeEach(func() {
				dnsPolicyBuilder.WithRoutingStrategy(v1alpha1.LoadBalancedRoutingStrategy)
				dnsPolicy = dnsPolicyBuilder.DNSPolicy
				Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
				Eventually(func() error { //dns policy exists
					return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
				}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
			})

			It("should create dns records", func() {
				Eventually(func(g Gomega, ctx context.Context) {
					recordList := &v1alpha1.DNSRecordList{}
					err := k8sClient.List(ctx, recordList, &client.ListOptions{Namespace: testNamespace})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(recordList.Items).To(HaveLen(2))
					g.Expect(recordList.Items).To(
						ContainElements(
							MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": HaveField("Name", recordName),
								"Spec": MatchFields(IgnoreExtras, Fields{
									"ManagedZoneRef": HaveField("Name", "mz-example-com"),
									"Endpoints": ConsistOf(
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal("19sc9b.lb-" + lbHash + ".test.example.com"),
											"Targets":       ConsistOf(TestIPAddressOne, TestIPAddressTwo),
											"RecordType":    Equal("A"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(60)),
										})),
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":          Equal("default.lb-" + lbHash + ".test.example.com"),
											"Targets":          ConsistOf("19sc9b.lb-" + lbHash + ".test.example.com"),
											"RecordType":       Equal("CNAME"),
											"SetIdentifier":    Equal("19sc9b.lb-" + lbHash + ".test.example.com"),
											"RecordTTL":        Equal(v1alpha1.TTL(60)),
											"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
										})),
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":          Equal("lb-" + lbHash + ".test.example.com"),
											"Targets":          ConsistOf("default.lb-" + lbHash + ".test.example.com"),
											"RecordType":       Equal("CNAME"),
											"SetIdentifier":    Equal("default"),
											"RecordTTL":        Equal(v1alpha1.TTL(300)),
											"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
										})),
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal(TestHostOne),
											"Targets":       ConsistOf("lb-" + lbHash + ".test.example.com"),
											"RecordType":    Equal("CNAME"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(300)),
										})),
									),
								}),
							}),
							MatchFields(IgnoreExtras, Fields{
								"ObjectMeta": HaveField("Name", wildcardRecordName),
								"Spec": MatchFields(IgnoreExtras, Fields{
									"ManagedZoneRef": HaveField("Name", "mz-example-com"),
									"Endpoints": ContainElements(
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal("19sc9b.lb-" + lbHash + ".example.com"),
											"Targets":       ConsistOf(TestIPAddressOne, TestIPAddressTwo),
											"RecordType":    Equal("A"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(60)),
										})),
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":          Equal("default.lb-" + lbHash + ".example.com"),
											"Targets":          ConsistOf("19sc9b.lb-" + lbHash + ".example.com"),
											"RecordType":       Equal("CNAME"),
											"SetIdentifier":    Equal("19sc9b.lb-" + lbHash + ".example.com"),
											"RecordTTL":        Equal(v1alpha1.TTL(60)),
											"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "weight", Value: "120"}}),
										})),
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":          Equal("lb-" + lbHash + ".example.com"),
											"Targets":          ConsistOf("default.lb-" + lbHash + ".example.com"),
											"RecordType":       Equal("CNAME"),
											"SetIdentifier":    Equal("default"),
											"RecordTTL":        Equal(v1alpha1.TTL(300)),
											"ProviderSpecific": Equal(v1alpha1.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
										})),
										PointTo(MatchFields(IgnoreExtras, Fields{
											"DNSName":       Equal(TestHostWildcard),
											"Targets":       ConsistOf("lb-" + lbHash + ".example.com"),
											"RecordType":    Equal("CNAME"),
											"SetIdentifier": Equal(""),
											"RecordTTL":     Equal(v1alpha1.TTL(300)),
										})),
									),
								}),
							}),
						))
				}, TestTimeoutMedium, TestRetryIntervalMedium, ctx).Should(Succeed())
			})

		})

	})

})
