//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("DNSPolicy Targeting an Istio Gateway", func() {

	// testID is a randomly generated identifier for the test
	// it is used to name resources and/or namespaces so different
	// tests can be run in parallel in the same cluster
	var testID string
	// testNamespace provided namespace in which to run tests (must contain provider secret)
	var testNamespace string
	// testZoneID provided zone id for the provider zone i.e. Route53 Hosted zone ID or GCP Managed zone Name
	var testZoneID string
	// testZoneDomainName provided domain name for the testZoneID e.g. e2e.hcpapps.net
	var testZoneDomainName string
	// testDomainName generated domain for this test e.g. t-e2e-12345.e2e.hcpapps.net
	var testDomainName string
	// testHostname generated hostname for this test e.g. t-dns-istio.t-e2e-12345.e2e.hcpapps.net
	var testHostname gatewayapiv1.Hostname

	var k8sClient client.Client

	var gw *gatewayapiv1.Gateway
	var httproute *gatewayapiv1.HTTPRoute
	var dnsPolicy *v1alpha2.DNSPolicy
	var mz *v1alpha2.ManagedZone

	BeforeEach(func(ctx SpecContext) {
		testID = "t-dns-" + tconfig.GenerateName()
		//ToDo Have this generate a new namespace instead of using the Hub Namespace and consider using a spoke client.
		// This currently still relies on the provider credentials secret being created ahead of time in a target namespace
		testNamespace = tconfig.HubNamespace()
		testZoneID = tconfig.DNSZoneID()
		testZoneDomainName = tconfig.DNSZoneDomainName()
		testDomainName = strings.Join([]string{testSuiteID, testZoneDomainName}, ".")
		testHostname = gatewayapiv1.Hostname(strings.Join([]string{testID, testDomainName}, "."))
		k8sClient = tconfig.HubClient()

		GinkgoWriter.Printf("[debug] testHostname: '%s'\n", testHostname)

		By("creating an Istio Gateway")
		gw = NewGatewayBuilder(testID, IstioGatewayClassName, testNamespace).
			WithListener(gatewayapiv1.Listener{
				Name:     "http",
				Hostname: &testHostname,
				Port:     80,
				Protocol: gatewayapiv1.HTTPProtocolType,
				AllowedRoutes: &gatewayapiv1.AllowedRoutes{
					Namespaces: &gatewayapiv1.RouteNamespaces{
						From: Pointer(gatewayapiv1.NamespacesFromAll),
					},
				},
			}).WithLabels(map[string]string{"gw": "t-e2e"}).Gateway
		err := k8sClient.Create(ctx, gw)
		Expect(err).ToNot(HaveOccurred())

		By("creating an HTTPRoute")
		httproute = &gatewayapiv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID,
				Namespace: testNamespace,
			},
			Spec: gatewayapiv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayapiv1.CommonRouteSpec{
					ParentRefs: []gatewayapiv1.ParentReference{{
						Namespace: Pointer(gatewayapiv1.Namespace(testNamespace)),
						Name:      gatewayapiv1.ObjectName(gw.GetName()),
						Kind:      Pointer(gatewayapiv1.Kind("Gateway")),
					}},
				},
				Hostnames: []gatewayapiv1.Hostname{testHostname},
				Rules: []gatewayapiv1.HTTPRouteRule{{
					BackendRefs: []gatewayapiv1.HTTPBackendRef{{
						BackendRef: gatewayapiv1.BackendRef{
							BackendObjectReference: gatewayapiv1.BackendObjectReference{
								Kind: Pointer(gatewayapiv1.Kind("Service")),
								Name: "test",
								Port: Pointer(gatewayapiv1.PortNumber(8080)),
							},
						},
					}},
				}},
			},
		}
		err = k8sClient.Create(ctx, httproute)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		if dnsPolicy != nil {
			err := k8sClient.Delete(ctx, dnsPolicy,
				client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
			Eventually(func(g Gomega) { // wait until it's gone to allow time for DNSRecords to be cleaned up
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsPolicy), dnsPolicy)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(ContainSubstring("not found")))

				recordList := &v1alpha2.DNSRecordList{}
				err = k8sClient.List(ctx, recordList, &client.MatchingLabels{"kuadrant.io/gateway": gw.GetName()}, &client.ListOptions{Namespace: testNamespace})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(recordList.Items).To(BeEmpty())
			}, TestTimeoutMedium, time.Second).Should(Succeed())
		}
		if mz != nil {
			err := k8sClient.Delete(ctx, mz,
				client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
		if httproute != nil {
			err := k8sClient.Delete(ctx, httproute,
				client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
		if gw != nil {
			err := k8sClient.Delete(ctx, gw,
				client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
	})

	Context("ManagedZone Provider", func() {

		BeforeEach(func(ctx SpecContext) {
			By("creating a ManagedZone")
			mz = NewManagedZoneBuilder(testID, testNamespace).
				WithID(testZoneID).
				WithDomainName(testDomainName).
				WithProviderSecret(tconfig.DNSProviderSecretName()).
				ManagedZone
			err := k8sClient.Create(ctx, mz)
			Expect(err).ToNot(HaveOccurred())
		})

		It("makes the hostname resolvable when a dnspolicy and httproute are attached", func(ctx SpecContext) {

			By("creating a DNSPolicy with ManagedZone provider")
			dnsPolicy = NewDNSPolicyBuilder(testID, testNamespace).
				WithTargetGateway(gw.GetName()).
				WithProviderManagedZone(mz.GetName()).
				WithRoutingStrategy(v1alpha2.SimpleRoutingStrategy).
				DNSPolicy
			err := k8sClient.Create(ctx, dnsPolicy)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsPolicy), dnsPolicy)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(dnsPolicy.Status.Conditions).To(
					ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(string(conditions.ConditionTypeReady)),
						"Status": Equal(metav1.ConditionTrue),
						"Reason": Equal("GatewayDNSEnabled"),
					})),
				)

				policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
				refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
				policiesBackRefValue := string(refs)
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(gw), gw)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gw.Annotations).To(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
				g.Expect(gw.Annotations).To(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))
			}, TestTimeoutMedium, time.Second).Should(Succeed())

			expectedRecordName := fmt.Sprintf("%s-%s", gw.GetName(), "http")
			Eventually(func(g Gomega) {
				dnsrecord := &v1alpha2.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedRecordName,
						Namespace: testNamespace,
					},
				}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsrecord), dnsrecord)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(dnsrecord.Status.Conditions).To(
					ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(string(conditions.ConditionTypeReady)),
						"Status": Equal(metav1.ConditionTrue),
					})),
				)
				g.Expect(dnsrecord.Spec.ZoneID).Should(PointTo(Equal(testZoneID)))
				g.Expect(dnsrecord.Spec.ProviderRef).Should(Equal(dnsPolicy.Spec.ProviderRef))
				g.Expect(dnsrecord.Spec.Endpoints).Should(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(string(testHostname)),
						"Targets":       Not(BeEmpty()),
						"RecordType":    Equal("A"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(v1alpha2.TTL(60)),
					})),
				))
				g.Expect(dnsrecord.Status.Endpoints).Should(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(string(testHostname)),
						"Targets":       Not(BeEmpty()),
						"RecordType":    Equal("A"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(v1alpha2.TTL(60)),
					})),
				))
			}, TestTimeoutLong, time.Second, ctx).Should(Succeed())

			By("ensuring the authoritative nameserver resolves the hostname")
			// speed up things by using the authoritative nameserver
			authoritativeResolver := ResolverForDomainName(testZoneDomainName)
			Eventually(func(ctx SpecContext) bool {
				c, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				IPs, err := authoritativeResolver.LookupHost(c, string(testHostname))
				if err != nil {
					GinkgoWriter.Printf("[debug] LookupHost error: '%s'\n", err)
				}
				return err == nil && len(IPs) > 0
			}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).Should(BeTrue())

		})
	})

	Context("Secret Provider", func() {

		It("makes the hostname resolvable when a dnspolicy and httproute are attached", func(ctx SpecContext) {

			By("creating a DNSPolicy with Secret provider")
			dnsPolicy = NewDNSPolicyBuilder(testID, testNamespace).
				WithTargetGateway(gw.GetName()).
				WithProviderSecret(tconfig.DNSProviderSecretName()).
				WithRoutingStrategy(v1alpha2.SimpleRoutingStrategy).
				DNSPolicy
			err := k8sClient.Create(ctx, dnsPolicy)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsPolicy), dnsPolicy)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(dnsPolicy.Status.Conditions).To(
					ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(string(conditions.ConditionTypeReady)),
						"Status": Equal(metav1.ConditionTrue),
						"Reason": Equal("GatewayDNSEnabled"),
					})),
				)

				policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
				refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
				policiesBackRefValue := string(refs)
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(gw), gw)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gw.Annotations).To(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
				g.Expect(gw.Annotations).To(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))
			}, TestTimeoutLong, time.Second).Should(Succeed())

			expectedRecordName := fmt.Sprintf("%s-%s", gw.GetName(), "http")
			Eventually(func(g Gomega) {
				dnsrecord := &v1alpha2.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedRecordName,
						Namespace: testNamespace,
					},
				}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsrecord), dnsrecord)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(dnsrecord.Status.Conditions).To(
					ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(string(conditions.ConditionTypeReady)),
						"Status": Equal(metav1.ConditionTrue),
					})),
				)
				g.Expect(dnsrecord.Spec.ZoneID).Should(PointTo(Equal(testZoneID)))
				g.Expect(dnsrecord.Spec.ProviderRef).Should(Equal(dnsPolicy.Spec.ProviderRef))
				g.Expect(dnsrecord.Spec.Endpoints).Should(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(string(testHostname)),
						"Targets":       Not(BeEmpty()),
						"RecordType":    Equal("A"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(v1alpha2.TTL(60)),
					})),
				))
				// We need to wait for the status to be updated otherwise google can leave things behind
				g.Expect(dnsrecord.Status.Endpoints).Should(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(string(testHostname)),
						"Targets":       Not(BeEmpty()),
						"RecordType":    Equal("A"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(v1alpha2.TTL(60)),
					})),
				))
			}, TestTimeoutLong, time.Second, ctx).Should(Succeed())

			By("ensuring the authoritative nameserver resolves the hostname")
			// speed up things by using the authoritative nameserver
			authoritativeResolver := ResolverForDomainName(testZoneDomainName)
			Eventually(func(ctx SpecContext) bool {
				c, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				IPs, err := authoritativeResolver.LookupHost(c, string(testHostname))
				if err != nil {
					GinkgoWriter.Printf("[debug] LookupHost error: '%s'\n", err)
				}
				return err == nil && len(IPs) > 0
			}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).Should(BeTrue())

		})
	})

	Context("None Provider", func() {

		It("should create dns record with no zone assigned and record should not become ready", func(ctx SpecContext) {

			By("creating a DNSPolicy with None provider")
			dnsPolicy = NewDNSPolicyBuilder(testID, testNamespace).
				WithTargetGateway(gw.GetName()).
				WithProviderNone("external-dns").
				WithRoutingStrategy(v1alpha2.SimpleRoutingStrategy).
				DNSPolicy
			err := k8sClient.Create(ctx, dnsPolicy)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsPolicy), dnsPolicy)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(dnsPolicy.Status.Conditions).To(
					ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(string(conditions.ConditionTypeReady)),
						"Status": Equal(metav1.ConditionTrue),
						"Reason": Equal("GatewayDNSEnabled"),
					})),
				)

				policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
				refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
				policiesBackRefValue := string(refs)
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(gw), gw)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gw.Annotations).To(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
				g.Expect(gw.Annotations).To(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))
			}, TestTimeoutLong, time.Second).Should(Succeed())

			expectedRecordName := fmt.Sprintf("%s-%s", gw.GetName(), "http")
			Eventually(func(g Gomega) {
				dnsrecord := &v1alpha2.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedRecordName,
						Namespace: testNamespace,
					},
				}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dnsrecord), dnsrecord)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(dnsrecord.Status.Conditions).ToNot(
					ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type": Equal(string(conditions.ConditionTypeReady)),
					})),
				)
				g.Expect(dnsrecord.Spec.ZoneID).Should(BeNil())
				g.Expect(dnsrecord.Spec.ProviderRef).Should(Equal(dnsPolicy.Spec.ProviderRef))
				g.Expect(dnsrecord.Spec.Endpoints).Should(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(string(testHostname)),
						"Targets":       Not(BeEmpty()),
						"RecordType":    Equal("A"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(v1alpha2.TTL(60)),
					})),
				))
				g.Expect(dnsrecord.Status.Endpoints).Should(BeEmpty())
			}, TestTimeoutLong, time.Second, ctx).Should(Succeed())

		})
	})

})
