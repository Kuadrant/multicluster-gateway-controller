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
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("DNSPolicy Single Cluster", Ordered, func() {

	var gatewayClass *gatewayv1beta1.GatewayClass
	var managedZone *v1alpha1.ManagedZone
	var testNamespace string
	var dnsPolicyBuilder *testutil.DNSPolicyBuilder
	var gateway *gatewayv1beta1.Gateway
	var dnsPolicy *v1alpha1.DNSPolicy
	var lbHash, recordName, wildcardRecordName string

	BeforeAll(func() {
		gatewayClass = testutil.NewTestGatewayClass("foo", "default", "kuadrant.io/bar")
		Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
		Eventually(func() error { // gateway class exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, gatewayClass)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, gatewayClass)
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		CreateNamespace(&testNamespace)

		managedZone = testutil.NewManagedZoneBuilder("mz-example-com", testNamespace, "example.com").ManagedZone
		Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())
		Eventually(func() error { // managed zone exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: managedZone.Name, Namespace: managedZone.Namespace}, managedZone)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

		gateway = testutil.NewGatewayBuilder(TestGatewayName, gatewayClass.Name, testNamespace).
			WithHTTPListener(TestListenerNameOne, TestHostOne).
			WithHTTPListener(TestListenerNameWildcard, TestHostWildcard).
			Gateway
		Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
		Eventually(func() error { //gateway exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		//Set single cluster gateway status
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

		dnsPolicyBuilder = testutil.NewDNSPolicyBuilder("test-dns-policy", testNamespace)
		dnsPolicyBuilder.WithTargetGateway(TestGatewayName)

		lbHash = dns.ToBase36hash(fmt.Sprintf("%s-%s", gateway.Name, gateway.Namespace))
		recordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameOne)
		wildcardRecordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameWildcard)
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
