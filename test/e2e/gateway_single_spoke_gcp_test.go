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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocm_cluster_v1beta1 "open-cluster-management.io/api/cluster/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	mgcv1alpha1 "github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("Gateway single target cluster GCP", func() {

	// testIDGCP is a randomly generated identifier for the test
	// it is used to name resources and/or namespaces so different
	// tests can be run in parallel in the same cluster
	var testIDGCP string
	var hostnameGCP gatewayapi.Hostname
	var gwGCP *gatewayapi.Gateway
	var placementGCP *ocm_cluster_v1beta1.Placement

	BeforeEach(func(ctx SpecContext) {
		testIDGCP = "t-e2e-" + tconfig.GenerateName()

		By("creating a Placement for the GCP Gateway resource ")
		placementGCP = &ocm_cluster_v1beta1.Placement{
			ObjectMeta: metav1.ObjectMeta{Name: testIDGCP, Namespace: tconfig.HubNamespace()},
			Spec: ocm_cluster_v1beta1.PlacementSpec{
				ClusterSets:      []string{tconfig.ManagedClusterSet()},
				NumberOfClusters: Pointer(int32(1)),
			},
		}

		err := tconfig.HubClient().Create(ctx, placementGCP)
		Expect(err).ToNot(HaveOccurred())

		By("creating a GCP Gateway in the hub")
		hname := strings.Replace(tconfig.ManagedZoneGCP(), "-", ".", -1)
		hostnameGCP = gatewayapi.Hostname(strings.Join([]string{testIDGCP, hname}, "."))
		gwGCP = &gatewayapi.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testIDGCP,
				Namespace: tconfig.HubNamespace(),
				Labels:    map[string]string{"gwGCP": "t-e2e"},
			},
			Spec: gatewayapi.GatewaySpec{
				GatewayClassName: GatewayClassName,
				Listeners: []gatewayapi.Listener{{
					Name:     "https",
					Hostname: &hostnameGCP,
					Port:     443,
					Protocol: gatewayapi.HTTPSProtocolType,
					TLS: &gatewayapi.GatewayTLSConfig{
						CertificateRefs: []gatewayapi.SecretObjectReference{{
							Name: gatewayapi.ObjectName(hostnameGCP),
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

		err = tconfig.HubClient().Create(ctx, gwGCP)
		Expect(err).ToNot(HaveOccurred())

		By("creating a GCP test application in the spoke for ")

		key := client.ObjectKey{Name: "test", Namespace: testIDGCP}

		err = tconfig.SpokeClient(0).Create(ctx, TestNamespace(key))
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.SpokeClient(0).Create(ctx, TestEchoDeployment(key))
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.SpokeClient(0).Create(ctx, TestEchoService(key))
		Expect(err).ToNot(HaveOccurred())

	})

	AfterEach(func(ctx SpecContext) {
		err := tconfig.HubClient().Delete(ctx, gwGCP,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.HubClient().Delete(ctx, placementGCP,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(err).ToNot(HaveOccurred())

		err = tconfig.SpokeClient(0).Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testIDGCP}},
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(err).ToNot(HaveOccurred())
	})

	When("the controller picks it up GCP", func() {

		It("sets the 'Accepted' conditions to true and programmed condition to unknown GCP", func(ctx SpecContext) {

			Eventually(func(ctx SpecContext) bool {
				err := tconfig.HubClient().Get(ctx, client.ObjectKey{Name: testIDGCP, Namespace: tconfig.HubNamespace()}, gwGCP)
				Expect(err).ToNot(HaveOccurred())
				programmed := meta.FindStatusCondition(gwGCP.Status.Conditions, string(gatewayapi.GatewayConditionProgrammed))
				return meta.IsStatusConditionTrue(gwGCP.Status.Conditions, string(gatewayapi.GatewayConditionAccepted)) && (nil != programmed && programmed.Status == "Unknown")

			}).WithContext(ctx).WithTimeout(30 * time.Second).WithPolling(10 * time.Second).Should(BeTrue())

		})
	})

	When("the placementGCP label is added to the GCP Gateway", func() {

		BeforeEach(func(ctx SpecContext) {
			By("adding a placementGCP label to the Gateway")
			patch := client.MergeFrom(gwGCP.DeepCopy())
			gwGCP.GetLabels()[PlacementLabel] = testIDGCP
			err := tconfig.HubClient().Patch(ctx, gwGCP, patch)
			Expect(err).ToNot(HaveOccurred())
		})

		It("it is placed on the spoke cluster GCP", func(ctx SpecContext) {

			istioGW := &gatewayapi.Gateway{}
			Eventually(func(ctx SpecContext) error {
				return tconfig.SpokeClient(0).Get(ctx, client.ObjectKey{Name: testIDGCP, Namespace: tconfig.SpokeNamespace()}, istioGW)
			}).WithContext(ctx).WithTimeout(60 * time.Second).WithPolling(10 * time.Second).ShouldNot(HaveOccurred())
		})

		When("an HTTPRoute is attached to the GCP Gateway", func() {
			var httproute *gatewayapi.HTTPRoute

			BeforeEach(func(ctx SpecContext) {

				By("attaching an HTTPRoute to the GCP Gateway in the spoke cluster")

				httproute = &gatewayapi.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: testIDGCP,
					},
					Spec: gatewayapi.HTTPRouteSpec{
						CommonRouteSpec: gatewayapi.CommonRouteSpec{
							ParentRefs: []gatewayapi.ParentReference{{
								Namespace: Pointer(gatewayapi.Namespace(tconfig.SpokeNamespace())),
								Name:      gatewayapi.ObjectName(testIDGCP),
								Kind:      Pointer(gatewayapi.Kind("Gateway")),
							}},
						},
						Hostnames: []gatewayapi.Hostname{hostnameGCP},
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

			It("makes available a GCP hostname that can be resolved and reachable through HTTPS", func(ctx SpecContext) {

				By("creating a GCP DNSPolicy in the hub")

				dnsPolicy := &mgcv1alpha1.DNSPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testIDGCP,
						Namespace: tconfig.HubNamespace(),
					},
					Spec: mgcv1alpha1.DNSPolicySpec{
						TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
							Group:     "gateway.networking.k8s.io",
							Kind:      "Gateway",
							Name:      gatewayapi.ObjectName(testIDGCP),
							Namespace: Pointer(gatewayapi.Namespace(tconfig.HubNamespace())),
						},
					},
				}
				err := tconfig.HubClient().Create(ctx, dnsPolicy)
				Expect(err).ToNot(HaveOccurred())

				// Wait for the DNSrecord to exists: this shouldn't be necessary but I have found out that GCP dns servers
				// cache the "no such host" response for a period of aprox 10-15 minutes, which makes the test run for a
				// long time.
				By("waiting for the GCP DNSRecord to be created in the Hub")

				Eventually(func(ctx SpecContext) bool {
					dnsrecord := &mgcv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-%s", gwGCP.Name, "https"),
						Namespace: gwGCP.Namespace,
					},
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

				By("ensuring the authoritative nameserver resolves the GCP hostname")

				// speed up things by using the authoritative nameserver
				nameservers, err := net.LookupNS(tconfig.ManagedZoneGCP())
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
					IPs, err := authoritativeResolver.LookupHost(c, string(hostnameGCP))
					if err != nil {
						GinkgoWriter.Printf("[debug] LooupHost error: '%s'\n", err)
					}
					return err == nil && len(IPs) > 0
				}).WithTimeout(300 * time.Second).WithPolling(10 * time.Second).WithContext(ctx).Should(BeTrue())

				By("performing a GET using HTTPS GCP")
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
						resp, err = httpClient.Get("https://" + string(hostnameGCP))
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
