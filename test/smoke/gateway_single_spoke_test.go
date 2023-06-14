//go:build e2e

package smoke

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	mgcv1alpha1 "github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ocm_cluster_v1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var _ = Describe("Gateway single target cluster", func() {

	// testID is a randomly generated identifier for the test
	// it is used to name resources and/or namespaces so different
	// tests can be run in parallel in the same cluster
	var testID string
	var hostname gatewayapi.Hostname
	var gw *gatewayapi.Gateway
	var placement *ocm_cluster_v1beta1.Placement

	BeforeEach(func(ctx SpecContext) {
		testID = "t-smoke-" + tconfig.GenerateName()

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
		hostname = gatewayapi.Hostname(strings.Join([]string{testID, tconfig.ManagedZone()}, "."))
		gw = &gatewayapi.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID,
				Namespace: tconfig.HubNamespace(),
				Labels:    map[string]string{"gw": "t-smoke"},
			},
			Spec: gatewayapi.GatewaySpec{
				GatewayClassName: GatewayClassName,
				Listeners: []gatewayapi.Listener{{
					Name:     "https",
					Hostname: &hostname,
					Port:     443,
					Protocol: gatewayapi.HTTPSProtocolType,
					TLS: &gatewayapi.GatewayTLSConfig{
						CertificateRefs: []gatewayapi.SecretObjectReference{{
							Name: gatewayapi.ObjectName(hostname),
						}},
					},
					AllowedRoutes: &gatewayapi.AllowedRoutes{
						Namespaces: &gatewayapi.RouteNamespaces{
							From: Pointer(gatewayapi.NamespacesFromAll),
						},
					},
				}},
			},
		}

		err = tconfig.HubClient().Create(ctx, gw)
		Expect(err).ToNot(HaveOccurred())

		By("creating a a test application in the spoke")

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
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.HubClient().Delete(ctx, placement,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.SpokeClient(0).Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testID}},
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(err).ToNot(HaveOccurred())
	})

	When("the controller picks it up", func() {

		It("sets the 'Programmed' and 'Accepted' conditions to true", func(ctx SpecContext) {

			Eventually(func(ctx SpecContext) bool {
				err := tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.HubNamespace()}, gw)
				Expect(err).ToNot(HaveOccurred())
				return meta.IsStatusConditionTrue(gw.Status.Conditions, string(gatewayapi.GatewayConditionAccepted)) &&
					meta.IsStatusConditionTrue(gw.Status.Conditions, string(gatewayapi.GatewayConditionProgrammed))
			}).WithContext(ctx).WithTimeout(30 * time.Second).WithPolling(10 * time.Second).Should(BeTrue())

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

		It("is placed on the spoke cluster", func(ctx SpecContext) {

			istioGW := &gatewayapi.Gateway{}
			Eventually(func(ctx SpecContext) error {
				return tconfig.SpokeClient(0).Get(ctx, client.ObjectKey{Name: testID, Namespace: tconfig.SpokeNamespace()}, istioGW)
			}).WithContext(ctx).WithTimeout(30 * time.Second).WithPolling(10 * time.Second).ShouldNot(HaveOccurred())
		})

		When("an HTTPRoute is attached to the Gateway", func() {
			var httproute *gatewayapi.HTTPRoute

			BeforeEach(func(ctx SpecContext) {

				By("attaching an HTTPRoute to the Gateway in the spoke cluster")

				httproute = &gatewayapi.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: testID,
					},
					Spec: gatewayapi.HTTPRouteSpec{
						CommonRouteSpec: gatewayapi.CommonRouteSpec{
							ParentRefs: []gatewayapi.ParentReference{{
								Namespace: Pointer(gatewayapi.Namespace(tconfig.SpokeNamespace())),
								Name:      gatewayapi.ObjectName(testID),
								Kind:      Pointer(gatewayapi.Kind("Gateway")),
							}},
						},
						Hostnames: []gatewayapi.Hostname{hostname},
						Rules: []gatewayapi.HTTPRouteRule{{
							BackendRefs: []gatewayapi.HTTPBackendRef{{
								BackendRef: gatewayapi.BackendRef{
									BackendObjectReference: gatewayapi.BackendObjectReference{
										Kind: Pointer(gatewayapi.Kind("Service")),
										Name: "test",
										Port: Pointer(gatewayapi.PortNumber(8080)),
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
				Expect(err).ToNot(HaveOccurred())
			})

			It("makes available a hostname that can be resolved and reachable through HTTPS", func(ctx SpecContext) {

				// Wait for the DNSrecord to exists: this shouldn't be necessary but I have found out that AWS dns servers
				// cache the "no such host" response for a period of aprox 10-15 minutes, which makes the test run for a
				// long time.
				By("waiting for the DNSRecord to be created in the Hub")

				Eventually(func(ctx SpecContext) bool {
					dnsrecord := &mgcv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
						Name: string(hostname), Namespace: tconfig.HubNamespace()},
					}
					if err := tconfig.HubClient().Get(ctx, client.ObjectKeyFromObject(dnsrecord), dnsrecord); err != nil {
						GinkgoWriter.Printf("[debug] unable to get DNSRecord: '%s'\n", err)
						return false
					}
					return meta.IsStatusConditionTrue(dnsrecord.Status.Conditions, conditions.ConditionTypeReady)
				}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).Should(BeTrue())

				// still need to wait some seconds to the dns server to actually start
				// resolving the hostname
				select {
				case <-ctx.Done():
				case <-time.After(30 * time.Second):
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
						GinkgoWriter.Printf("[debug] LooupHost error: '%s'\n", err)
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
					httpClient := &http.Client{}

					var resp *http.Response
					Eventually(func(ctx SpecContext) error {
						resp, err = httpClient.Get("https://" + string(hostname))
						if err != nil {
							GinkgoWriter.Printf("[debug] GET error: '%s'\n", err)
							return err
						}
						return nil
					}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).ShouldNot(HaveOccurred())

					defer resp.Body.Close()
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
				}
			})

		})

	})

})
