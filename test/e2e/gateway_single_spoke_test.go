//go:build e2e

package e2e

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	certmanmetav1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	ocm_cluster_v1beta1 "open-cluster-management.io/api/cluster/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	mgcv1alpha1 "github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("Gateway single target cluster", func() {

	// testID is a randomly generated identifier for the test
	// it is used to name resources and/or namespaces so different
	// tests can be run in parallel in the same cluster
	var testID string
	var hostname gatewayapiv1.Hostname
	var otherHostname gatewayapiv1.Hostname
	var wildcardHostname gatewayapiv1.Hostname
	var gw *gatewayapiv1.Gateway
	var placement *ocm_cluster_v1beta1.Placement
	var tlsPolicy *v1alpha1.TLSPolicy

	BeforeEach(func(ctx SpecContext) {
		testID = "t-e2e-" + tconfig.GenerateName()

		By("creating a Placement for the Gateway resource")
		placement = &ocm_cluster_v1beta1.Placement{
			ObjectMeta: metav1.ObjectMeta{Name: testID, Namespace: tconfig.HubNamespace()},
			Spec: ocm_cluster_v1beta1.PlacementSpec{
				ClusterSets:      []string{tconfig.ManagedClusterSet()},
				NumberOfClusters: Pointer(int32(1)),
			},
		}

		err := tconfig.HubClient().Create(ctx, placement)
		Expect(err).ToNot(HaveOccurred())

		By("creating a Gateway in the hub")
		hostname = gatewayapiv1.Hostname(strings.Join([]string{testID, tconfig.ManagedZone()}, "."))
		gw = NewGatewayBuilder(testID, GatewayClassName, tconfig.HubNamespace()).WithListener(gatewayapiv1.Listener{
			Name:     "https",
			Hostname: &hostname,
			Port:     443,
			Protocol: gatewayapiv1.HTTPSProtocolType,
			TLS: &gatewayapiv1.GatewayTLSConfig{
				CertificateRefs: []gatewayapiv1.SecretObjectReference{{
					Name: gatewayapiv1.ObjectName(hostname),
				}},
			},
			AllowedRoutes: &gatewayapiv1.AllowedRoutes{
				Namespaces: &gatewayapiv1.RouteNamespaces{
					From: Pointer(gatewayapiv1.NamespacesFromAll),
				},
			},
		}).WithLabels(map[string]string{"gw": "t-e2e"}).Gateway
		err = tconfig.HubClient().Create(ctx, gw)
		Expect(err).ToNot(HaveOccurred())

		By("setting up TLSPolicy in the hub")
		tlsPolicy = &mgcv1alpha1.TLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID,
				Namespace: tconfig.HubNamespace(),
			},
			Spec: mgcv1alpha1.TLSPolicySpec{
				TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
					Group:     "gateway.networking.k8s.io",
					Kind:      "Gateway",
					Name:      gatewayapiv1.ObjectName(testID),
					Namespace: Pointer(gatewayapiv1.Namespace(tconfig.HubNamespace())),
				},
				CertificateSpec: mgcv1alpha1.CertificateSpec{
					IssuerRef: certmanmetav1.ObjectReference{
						Name:  "glbc-ca",
						Kind:  "ClusterIssuer",
						Group: "cert-manager.io",
					},
				},
			},
		}
		err = tconfig.HubClient().Create(ctx, tlsPolicy)
		Expect(err).ToNot(HaveOccurred())

		By("creating a test application in the spoke")

		key := client.ObjectKey{Name: "test", Namespace: testID}

		err = tconfig.SpokeClient(0).Create(ctx, TestNamespace(key))
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.SpokeClient(0).Create(ctx, TestEchoDeployment(key))
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.SpokeClient(0).Create(ctx, TestEchoService(key))
		Expect(err).ToNot(HaveOccurred())

	})

	AfterEach(func(ctx SpecContext) {
		err := tconfig.HubClient().Delete(ctx, gw,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

		err = tconfig.HubClient().Delete(ctx, tlsPolicy,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

		//Workaround for https://github.com/Kuadrant/multicluster-gateway-controller/issues/420
		Eventually(func(ctx SpecContext) error {
			return tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
		}).WithContext(ctx).WithTimeout(10 * time.Second).WithPolling(2 * time.Second).Should(MatchError(ContainSubstring("not found")))

		err = tconfig.HubClient().Delete(ctx, placement,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

		err = tconfig.SpokeClient(0).Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testID}},
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
	})

	When("the controller picks the gateway up", func() {

		It("sets the 'Accepted' conditions to true and programmed condition to unknown", func(ctx SpecContext) {

			Eventually(func(ctx SpecContext) error {
				return tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
			}).WithContext(ctx).WithTimeout(10 * time.Second).WithPolling(2 * time.Second).ShouldNot(HaveOccurred())

			Eventually(func(ctx SpecContext) error {
				err := tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
				Expect(err).ToNot(HaveOccurred())
				if !meta.IsStatusConditionPresentAndEqual(gw.Status.Conditions, string(gatewayapiv1.GatewayConditionProgrammed), "Unknown") {
					cond := meta.FindStatusCondition(gw.Status.Conditions, string(gatewayapiv1.GatewayConditionProgrammed))
					return fmt.Errorf("expected condition %s to be Unknown but got %v", string(gatewayapiv1.GatewayConditionProgrammed), cond)
				}
				return nil
			}).WithContext(ctx).WithTimeout(10 * time.Second).WithPolling(2 * time.Second).ShouldNot(HaveOccurred())

			Eventually(func(ctx SpecContext) error {
				err := tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
				Expect(err).ToNot(HaveOccurred())
				if !meta.IsStatusConditionTrue(gw.Status.Conditions, string(gatewayapiv1.GatewayConditionAccepted)) {
					return fmt.Errorf("expected condition %s to be true", string(gatewayapiv1.GatewayConditionAccepted))
				}
				return nil
			}).WithContext(ctx).WithTimeout(10 * time.Second).WithPolling(2 * time.Second).ShouldNot(HaveOccurred())

		})
	})

	When("the Placement label is added to the Gateway", func() {

		BeforeEach(func(ctx SpecContext) {
			By("adding a placement label to the Gateway")
			patch := client.MergeFrom(gw.DeepCopy())
			gw.GetLabels()[PlacementLabel] = testID
			err := tconfig.HubClient().Patch(ctx, gw, patch)
			Expect(err).ToNot(HaveOccurred())
		})

		It("the gateway is placed on the spoke cluster once the tls secrets exist", func(ctx SpecContext) {
			istioGW := &gatewayapiv1.Gateway{}
			Eventually(func(ctx SpecContext) error {
				return tconfig.SpokeClient(0).Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.SpokeNamespace()}, istioGW)
			}).WithContext(ctx).WithTimeout(120 * time.Second).WithPolling(10 * time.Second).ShouldNot(HaveOccurred())
		})

		When("an HTTPRoute is attached to the Gateway", func() {
			var httproute *gatewayapiv1.HTTPRoute

			BeforeEach(func(ctx SpecContext) {

				By("attaching an HTTPRoute to the Gateway in the spoke cluster")

				httproute = &gatewayapiv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: testID,
					},
					Spec: gatewayapiv1.HTTPRouteSpec{
						CommonRouteSpec: gatewayapiv1.CommonRouteSpec{
							ParentRefs: []gatewayapiv1.ParentReference{{
								Namespace: Pointer(gatewayapiv1.Namespace(tconfig.SpokeNamespace())),
								Name:      gatewayapiv1.ObjectName(testID),
								Kind:      Pointer(gatewayapiv1.Kind("Gateway")),
							}},
						},
						Hostnames: []gatewayapiv1.Hostname{hostname},
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

				err := tconfig.SpokeClient(0).Create(ctx, httproute)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func(ctx SpecContext) {
				err := tconfig.SpokeClient(0).Delete(ctx, httproute)
				Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
			})

			When("a DNSPolicy is attached to the Gateway", func() {
				var dnsPolicy *mgcv1alpha1.DNSPolicy

				BeforeEach(func(ctx SpecContext) {

					By("creating a DNSPolicy in the hub")

					dnsPolicy = &mgcv1alpha1.DNSPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      testID,
							Namespace: tconfig.HubNamespace(),
						},
						Spec: mgcv1alpha1.DNSPolicySpec{
							TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
								Group:     "gateway.networking.k8s.io",
								Kind:      "Gateway",
								Name:      gatewayapiv1.ObjectName(testID),
								Namespace: Pointer(gatewayapiv1.Namespace(tconfig.HubNamespace())),
							},
							RoutingStrategy: v1alpha1.LoadBalancedRoutingStrategy,
						},
					}
					err := tconfig.HubClient().Create(ctx, dnsPolicy)
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func(ctx SpecContext) {
					err := tconfig.HubClient().Delete(ctx, dnsPolicy)
					Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      strings.Join([]string{testID, tconfig.ManagedZone()}, "."),
							Namespace: tconfig.HubNamespace(),
						},
					}
					err = tconfig.HubClient().Delete(ctx, secret)
					Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
					cert := &certmanv1.Certificate{
						ObjectMeta: metav1.ObjectMeta{
							Name:      strings.Join([]string{testID, tconfig.ManagedZone()}, "."),
							Namespace: tconfig.HubNamespace(),
						},
					}
					err = tconfig.HubClient().Delete(ctx, cert)
					Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
				})

				It("makes available a hostname that can be resolved and reachable through HTTPS", func(ctx SpecContext) {

					// Wait for the DNSRecord to exists: this shouldn't be necessary, but I have found out that AWS dns servers
					// cache the "no such host" response for a period of approx 10-15 minutes, which makes the test run for a
					// long time.
					By("waiting for the DNSRecord to be created in the Hub")
					{
						Eventually(func(ctx SpecContext) bool {
							dnsrecord := &mgcv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
								Name:      fmt.Sprintf("%s-%s", gw.Name, "https"),
								Namespace: gw.Namespace,
							},
							}
							if err := tconfig.HubClient().Get(ctx, client.ObjectKeyFromObject(dnsrecord), dnsrecord); err != nil {
								GinkgoWriter.Printf("[debug] unable to get DNSRecord: '%s'\n", err)
								return false
							}
							return meta.IsStatusConditionTrue(dnsrecord.Status.Conditions, string(conditions.ConditionTypeReady))
						}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).Should(BeTrue())

						// still need to wait some seconds to the dns server to actually start
						// resolving the hostname
						select {
						case <-ctx.Done():
						case <-time.After(30 * time.Second):
						}
					}
					By("ensuring the authoritative nameserver resolves the hostname")

					// speed up things by using the authoritative nameserver
					nameservers, err := net.LookupNS(tconfig.ManagedZone())
					Expect(err).ToNot(HaveOccurred())
					GinkgoWriter.Printf("[debug] authoritative nameserver used for DNS record resolution: %s\n", nameservers[0].Host)

					authoritativeResolver := &net.Resolver{
						PreferGo: true,
						Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
							d := net.Dialer{Timeout: 10 * time.Second}
							return d.DialContext(ctx, network, strings.Join([]string{nameservers[0].Host, "53"}, ":"))
						},
					}

					Eventually(func(ctx SpecContext) bool {
						c, cancel := context.WithTimeout(ctx, 10*time.Second)
						defer cancel()
						IPs, err := authoritativeResolver.LookupHost(c, string(hostname))
						if err != nil {
							GinkgoWriter.Printf("[debug] LookupHost error: '%s'\n", err)
						}
						return err == nil && len(IPs) > 0
					}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).Should(BeTrue())

					By("performing a GET using HTTPS")
					{
						// use the authoritative nameservers
						dialer := &net.Dialer{Resolver: authoritativeResolver}
						dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
							return dialer.DialContext(ctx, network, addr)
						}
						http.DefaultTransport.(*http.Transport).DialContext = dialContext
						http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

						var resp *http.Response
						Eventually(func(g Gomega, ctx SpecContext) {
							httpClient := &http.Client{}
							resp, err = httpClient.Get("https://" + string(hostname))
							g.Expect(err).ToNot(HaveOccurred())
						}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).Should(Succeed())

						defer resp.Body.Close()
						Expect(resp.StatusCode).To(Equal(http.StatusOK))
					}
					By("adding wildcard listener to the gateway")
					{
						gw := &gatewayapiv1.Gateway{}
						err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
						Expect(err).ToNot(HaveOccurred())

						if gw.Spec.Listeners == nil {
							gw.Spec.Listeners = []gatewayapiv1.Listener{}
						}
						wildcardHostname = gatewayapiv1.Hostname(strings.Join([]string{"*", tconfig.ManagedZone()}, "."))
						secretName := gatewayapiv1.Hostname(strings.Join([]string{testID, tconfig.ManagedZone()}, "."))
						AddListener("wildcard", wildcardHostname, gatewayapiv1.ObjectName(secretName), gw)
						err = tconfig.HubClient().Update(ctx, gw)
						Expect(err).ToNot(HaveOccurred())
						Eventually(func(g Gomega, ctx SpecContext) {
							checkGateway := &gatewayapiv1.Gateway{}
							err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, checkGateway)
							g.Expect(err).ToNot(HaveOccurred())
							g.Expect(len(checkGateway.Spec.Listeners)).To(Equal(2))
						}).WithContext(ctx).WithTimeout(100 * time.Second).WithPolling(2 * time.Second).Should(Succeed())

					}

					By("checking tls secrets for wildcard entry in annotation")
					{
						secretList := &corev1.SecretList{}
						Eventually(func(g Gomega, ctx SpecContext) {
							err = tconfig.HubClient().List(ctx, secretList)
							g.Expect(err).ToNot(HaveOccurred())
							g.Expect(secretList.Items).To(Not(BeEmpty()))
							g.Expect(secretList.Items).To(
								ContainElement(
									MatchFields(IgnoreExtras, Fields{
										"ObjectMeta": MatchFields(IgnoreExtras, Fields{
											"Annotations": HaveKeyWithValue("cert-manager.io/alt-names", fmt.Sprintf("%s,%s", string(hostname), string(wildcardHostname))),
										}),
									})),
							)
						}).WithContext(ctx).WithTimeout(180 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
					}

					By("checking tls certificate")
					{
						certList := &certmanv1.CertificateList{}
						Eventually(func(g Gomega, ctx SpecContext) {
							err = tconfig.HubClient().List(ctx, certList)
							g.Expect(err).NotTo(HaveOccurred())
							g.Expect(certList.Items).To(Not(BeEmpty()))
							g.Expect(certList.Items).To(
								ContainElement(
									MatchFields(IgnoreExtras, Fields{
										"ObjectMeta": MatchFields(IgnoreExtras, Fields{
											"Labels": HaveKeyWithValue("gateway", testID),
										}),
										"Spec": MatchFields(IgnoreExtras, Fields{
											"DNSNames": ConsistOf(string(hostname), string(wildcardHostname)),
										}),
									})))
						}).WithContext(ctx).WithTimeout(180 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
					}
					By("when adding/removing listeners, checking that tls secrets are added/removed")
					{
						gw := &gatewayapiv1.Gateway{}
						err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
						Expect(err).ToNot(HaveOccurred())

						if gw.Spec.Listeners == nil {
							gw.Spec.Listeners = []gatewayapiv1.Listener{}
						}
						otherHostname = gatewayapiv1.Hostname(strings.Join([]string{"other", tconfig.ManagedZone()}, "."))
						AddListener("other", otherHostname, gatewayapiv1.ObjectName(otherHostname), gw)
						Eventually(func(g Gomega, ctx SpecContext) {
							err = tconfig.HubClient().Update(ctx, gw)
							Expect(err).ToNot(HaveOccurred())
							checkGateway := &gatewayapiv1.Gateway{}
							err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, checkGateway)
							g.Expect(err).ToNot(HaveOccurred())
							g.Expect(len(checkGateway.Spec.Listeners)).To(Equal(3))
						}).WithContext(ctx).WithTimeout(100 * time.Second).WithPolling(2 * time.Second).Should(Succeed())

						Eventually(func(g Gomega, ctx SpecContext) {
							secret := &corev1.Secret{}
							err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: string(otherHostname), Namespace: tconfig.HubNamespace()}, secret)
							g.Expect(err).ToNot(HaveOccurred())
						}).WithContext(ctx).WithTimeout(180 * time.Second).WithPolling(2 * time.Second).Should(Succeed())

						// remove the listener
						err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
						Expect(err).ToNot(HaveOccurred())

						if gw.Spec.Listeners == nil {
							gw.Spec.Listeners = []gatewayapiv1.Listener{}
						}

						for i, listener := range gw.Spec.Listeners {
							if listener.Name == "other" {
								gw.Spec.Listeners = append(gw.Spec.Listeners[:i], gw.Spec.Listeners[i+1:]...)
							}
						}
						err = tconfig.HubClient().Update(ctx, gw)
						Expect(err).ToNot(HaveOccurred())
						Eventually(func(g Gomega, ctx SpecContext) {
							secret := &corev1.Secret{}
							err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: string(otherHostname), Namespace: tconfig.HubNamespace()}, secret)
							g.Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
						}).WithContext(ctx).WithTimeout(180 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
					}
					By("when deleting tls policy, checking that tls secrets are removed")
					{
						tlsPolicy = &mgcv1alpha1.TLSPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      testID,
								Namespace: tconfig.HubNamespace(),
							},
						}
						err := tconfig.HubClient().Delete(ctx, tlsPolicy,
							client.PropagationPolicy(metav1.DeletePropagationForeground))
						Expect(err).ToNot(HaveOccurred())
						hostname = gatewayapiv1.Hostname(strings.Join([]string{testID, tconfig.ManagedZone()}, "."))
						Eventually(func(g Gomega, ctx SpecContext) {
							secret := &corev1.Secret{}
							err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: string(hostname), Namespace: tconfig.HubNamespace()}, secret)
							g.Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
						}).WithContext(ctx).WithTimeout(180 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
					}
				})
			})
		})
	})
})
