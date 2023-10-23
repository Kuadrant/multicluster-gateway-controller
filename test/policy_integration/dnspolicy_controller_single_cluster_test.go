//go:build integration

package policy_integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("DNSPolicy Single Cluster", func() {

	var gatewayClass *gatewayapiv1.GatewayClass
	var managedZone *v1alpha2.ManagedZone
	var testNamespace string
	var dnsPolicyBuilder *testutil.DNSPolicyBuilder
	var gateway *gatewayapiv1.Gateway
	var dnsPolicy *v1alpha2.DNSPolicy
	var lbHash, recordName, wildcardRecordName string

	BeforeEach(func() {
		CreateNamespace(&testNamespace)

		gatewayClass = testutil.NewTestGatewayClass("foo", "default", "kuadrant.io/bar")
		Expect(k8sClient.Create(ctx, gatewayClass)).To(Succeed())

		managedZone = testutil.NewManagedZoneBuilder("mz-example-com", testNamespace).
			WithID("1234").
			WithDomainName("example.com").
			WithDescription("example.com").
			WithProviderSecret("secretname").
			ManagedZone
		Expect(k8sClient.Create(ctx, managedZone)).To(Succeed())

		gateway = testutil.NewGatewayBuilder(TestGatewayName, gatewayClass.Name, testNamespace).
			WithHTTPListener(TestListenerNameOne, TestHostOne).
			WithHTTPListener(TestListenerNameWildcard, TestHostWildcard).
			Gateway
		Expect(k8sClient.Create(ctx, gateway)).To(Succeed())

		//Set single cluster gateway status
		Eventually(func() error {
			gateway.Status.Addresses = []gatewayapiv1.GatewayStatusAddress{
				{
					Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
					Value: TestIPAddressOne,
				},
				{
					Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
					Value: TestIPAddressTwo,
				},
			}
			gateway.Status.Listeners = []gatewayapiv1.ListenerStatus{
				{
					Name:           TestListenerNameOne,
					SupportedKinds: []gatewayapiv1.RouteGroupKind{},
					AttachedRoutes: 1,
					Conditions:     []metav1.Condition{},
				},
				{
					Name:           TestListenerNameWildcard,
					SupportedKinds: []gatewayapiv1.RouteGroupKind{},
					AttachedRoutes: 1,
					Conditions:     []metav1.Condition{},
				},
			}
			return k8sClient.Status().Update(ctx, gateway)
		}, TestTimeoutMedium, TestRetryIntervalMedium).Should(Succeed())

		dnsPolicyBuilder = testutil.NewDNSPolicyBuilder("test-dns-policy", testNamespace).
			WithProviderManagedZone(managedZone.Name).
			WithTargetGateway(TestGatewayName)

		lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))
		recordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameOne)
		wildcardRecordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameWildcard)
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
		if managedZone != nil {
			err := k8sClient.Delete(ctx, managedZone)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
		if gatewayClass != nil {
			err := k8sClient.Delete(ctx, gatewayClass)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
	})

	Context("simple routing strategy", func() {

		BeforeEach(func() {
			dnsPolicyBuilder.WithRoutingStrategy(v1alpha2.SimpleRoutingStrategy)
			dnsPolicy = dnsPolicyBuilder.DNSPolicy
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(Succeed())
		})

		It("should create dns records", func() {
			Eventually(func(g Gomega, ctx context.Context) {
				recordList := &v1alpha2.DNSRecordList{}
				err := k8sClient.List(ctx, recordList, &client.ListOptions{Namespace: testNamespace})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(recordList.Items).To(HaveLen(2))
				g.Expect(recordList.Items).To(
					ContainElements(
						MatchFields(IgnoreExtras, Fields{
							"ObjectMeta": HaveField("Name", recordName),
							"Spec": MatchFields(IgnoreExtras, Fields{
								"ZoneID":      Equal(managedZone.Spec.ID),
								"ProviderRef": Equal(dnsPolicy.Spec.ProviderRef),
								"Endpoints": ConsistOf(
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":       Equal(TestHostOne),
										"Targets":       ContainElements(TestIPAddressOne, TestIPAddressTwo),
										"RecordType":    Equal("A"),
										"SetIdentifier": Equal(""),
										"RecordTTL":     Equal(v1alpha2.TTL(60)),
									})),
								),
							}),
						}),
						MatchFields(IgnoreExtras, Fields{
							"ObjectMeta": HaveField("Name", wildcardRecordName),
							"Spec": MatchFields(IgnoreExtras, Fields{
								"ZoneID":      Equal(managedZone.Spec.ID),
								"ProviderRef": Equal(dnsPolicy.Spec.ProviderRef),
								"Endpoints": ConsistOf(
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":       Equal(TestHostWildcard),
										"Targets":       ContainElements(TestIPAddressOne, TestIPAddressTwo),
										"RecordType":    Equal("A"),
										"SetIdentifier": Equal(""),
										"RecordTTL":     Equal(v1alpha2.TTL(60)),
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
			dnsPolicyBuilder.WithRoutingStrategy(v1alpha2.LoadBalancedRoutingStrategy)
			dnsPolicy = dnsPolicyBuilder.DNSPolicy
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(Succeed())
		})

		It("should create dns records", func() {
			Eventually(func(g Gomega, ctx context.Context) {
				recordList := &v1alpha2.DNSRecordList{}
				err := k8sClient.List(ctx, recordList, &client.ListOptions{Namespace: testNamespace})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(recordList.Items).To(HaveLen(2))
				g.Expect(recordList.Items).To(
					ContainElements(
						MatchFields(IgnoreExtras, Fields{
							"ObjectMeta": HaveField("Name", recordName),
							"Spec": MatchFields(IgnoreExtras, Fields{
								"ZoneID":      Equal(managedZone.Spec.ID),
								"ProviderRef": Equal(dnsPolicy.Spec.ProviderRef),
								"Endpoints": ConsistOf(
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":       Equal("19sc9b.lb-" + lbHash + ".test.example.com"),
										"Targets":       ConsistOf(TestIPAddressOne, TestIPAddressTwo),
										"RecordType":    Equal("A"),
										"SetIdentifier": Equal(""),
										"RecordTTL":     Equal(v1alpha2.TTL(60)),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":          Equal("default.lb-" + lbHash + ".test.example.com"),
										"Targets":          ConsistOf("19sc9b.lb-" + lbHash + ".test.example.com"),
										"RecordType":       Equal("CNAME"),
										"SetIdentifier":    Equal("19sc9b.lb-" + lbHash + ".test.example.com"),
										"RecordTTL":        Equal(v1alpha2.TTL(60)),
										"ProviderSpecific": Equal(v1alpha2.ProviderSpecific{{Name: "weight", Value: "120"}}),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":          Equal("lb-" + lbHash + ".test.example.com"),
										"Targets":          ConsistOf("default.lb-" + lbHash + ".test.example.com"),
										"RecordType":       Equal("CNAME"),
										"SetIdentifier":    Equal("default"),
										"RecordTTL":        Equal(v1alpha2.TTL(300)),
										"ProviderSpecific": Equal(v1alpha2.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":       Equal(TestHostOne),
										"Targets":       ConsistOf("lb-" + lbHash + ".test.example.com"),
										"RecordType":    Equal("CNAME"),
										"SetIdentifier": Equal(""),
										"RecordTTL":     Equal(v1alpha2.TTL(300)),
									})),
								),
							}),
						}),
						MatchFields(IgnoreExtras, Fields{
							"ObjectMeta": HaveField("Name", wildcardRecordName),
							"Spec": MatchFields(IgnoreExtras, Fields{
								"ZoneID":      Equal(managedZone.Spec.ID),
								"ProviderRef": Equal(dnsPolicy.Spec.ProviderRef),
								"Endpoints": ContainElements(
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":       Equal("19sc9b.lb-" + lbHash + ".example.com"),
										"Targets":       ConsistOf(TestIPAddressOne, TestIPAddressTwo),
										"RecordType":    Equal("A"),
										"SetIdentifier": Equal(""),
										"RecordTTL":     Equal(v1alpha2.TTL(60)),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":          Equal("default.lb-" + lbHash + ".example.com"),
										"Targets":          ConsistOf("19sc9b.lb-" + lbHash + ".example.com"),
										"RecordType":       Equal("CNAME"),
										"SetIdentifier":    Equal("19sc9b.lb-" + lbHash + ".example.com"),
										"RecordTTL":        Equal(v1alpha2.TTL(60)),
										"ProviderSpecific": Equal(v1alpha2.ProviderSpecific{{Name: "weight", Value: "120"}}),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":          Equal("lb-" + lbHash + ".example.com"),
										"Targets":          ConsistOf("default.lb-" + lbHash + ".example.com"),
										"RecordType":       Equal("CNAME"),
										"SetIdentifier":    Equal("default"),
										"RecordTTL":        Equal(v1alpha2.TTL(300)),
										"ProviderSpecific": Equal(v1alpha2.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"DNSName":       Equal(TestHostWildcard),
										"Targets":       ConsistOf("lb-" + lbHash + ".example.com"),
										"RecordType":    Equal("CNAME"),
										"SetIdentifier": Equal(""),
										"RecordTTL":     Equal(v1alpha2.TTL(300)),
									})),
								),
							}),
						}),
					))
			}, TestTimeoutMedium, TestRetryIntervalMedium, ctx).Should(Succeed())
		})

	})

})
