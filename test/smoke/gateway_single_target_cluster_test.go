package smoke

import (
	"context"
	"strings"
	"time"

	. "github.com/Kuadrant/multi-cluster-traffic-controller/test/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var _ = Describe("Gateway single target cluster", func() {

	// gwname is the name of the Gateway that will be used
	// as the test subject. It is a generated name to allow for
	// test parallelism
	var gwname string
	var hostname gatewayapi.Hostname
	var ctx context.Context = context.Background()

	BeforeEach(func() {

		// NOTE This will only be useful once we have multi-teanancy and can create gateways
		// in different tenant namespaces
		//
		// By("creating a tenant namespace in the control plane")
		// testNamespace = "test-ns-" + nameGenerator.Generate()
		// ns := &corev1.Namespace{
		// 	TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		// 	ObjectMeta: metav1.ObjectMeta{Name: testNamespace},
		// }
		// err := k8sClient.Create(context.Background(), ns)
		// Expect(err).ToNot(HaveOccurred())
		// n := &corev1.Namespace{}
		// Eventually(func() bool {
		// 	err := k8sClient.Get(context.Background(), types.NamespacedName{Name: testNamespace}, n)
		// 	return err == nil
		// }, 60*time.Second, 5*time.Second).Should(BeTrue())

		By("creating a Gateway in the control plane")
		gwname = "t-smoke-" + tconfig.GenerateName()

		hostname = gatewayapi.Hostname(strings.Join([]string{gwname, tconfig.ManagedZone()}, "."))
		gw := &gatewayapi.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:        gwname,
				Namespace:   tconfig.TenantNamespace(),
				Annotations: map[string]string{ClusterSelectorLabelKey: ClusterSelectorLabelValue},
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

		err := tconfig.ControlPlaneClient().Create(context.Background(), gw)
		Expect(err).ToNot(HaveOccurred())

	})

	AfterEach(func() {

		gw := &gatewayapi.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gwname,
				Namespace: tconfig.TenantNamespace(),
			},
		}
		err := tconfig.ControlPlaneClient().Delete(context.Background(), gw, client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(err).ToNot(HaveOccurred())
	})

	When("the controller picks it up", func() {

		It("sets the 'Accepted' condition to true", func() {

			gw := &gatewayapi.Gateway{}
			Eventually(func() bool {
				if err := tconfig.ControlPlaneClient().Get(ctx, types.NamespacedName{Name: gwname, Namespace: tconfig.TenantNamespace()}, gw); err != nil {
					return false
				}
				return meta.IsStatusConditionTrue(gw.Status.Conditions, string(gatewayapi.GatewayConditionAccepted))
			}, 60*time.Second, 5*time.Second).Should(BeTrue())
		})
	})

	When("an HTTPRoute is attached ot the Gateway", func() {
		var ns *corev1.Namespace
		var route *gatewayapi.HTTPRoute

		BeforeEach(func() {

			By("creating a test namespace in the dataplane cluster")
			nsname := "t-smoke-dataplane-" + tconfig.GenerateName()
			ns = &corev1.Namespace{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{Name: nsname},
			}
			err := tconfig.DataPlaneClient(0).Create(context.Background(), ns)
			Expect(err).ToNot(HaveOccurred())

			By("attaching an HTTPRoute to the Gateway in the dataplane")

			route = &gatewayapi.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns.GetName(),
				},
				Spec: gatewayapi.HTTPRouteSpec{
					CommonRouteSpec: gatewayapi.CommonRouteSpec{
						ParentRefs: []gatewayapi.ParentReference{{
							Namespace: Pointer(gatewayapi.Namespace(tconfig.DataplaneConfigNamespace())),
							Name:      gatewayapi.ObjectName(gwname),
						}},
					},
					Hostnames: []gatewayapi.Hostname{hostname},
					Rules: []gatewayapi.HTTPRouteRule{{
						Matches: []gatewayapi.HTTPRouteMatch{{
							Path: &gatewayapi.HTTPPathMatch{
								Type:  Pointer(gatewayapi.PathMatchPathPrefix),
								Value: Pointer("/"),
							},
							Method: Pointer(gatewayapi.HTTPMethodGet),
						}},
						BackendRefs: []gatewayapi.HTTPBackendRef{{
							BackendRef: gatewayapi.BackendRef{
								BackendObjectReference: gatewayapi.BackendObjectReference{
									Group: Pointer(gatewayapi.Group("")),
									Kind:  Pointer(gatewayapi.Kind("Service")),
									Name:  "test",
									Port:  Pointer(gatewayapi.PortNumber(80)),
								},
								Weight: Pointer(int32(1)),
							},
						}},
					}},
				},
			}

			err = tconfig.DataPlaneClient(0).Create(context.Background(), route)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := tconfig.DataPlaneClient(0).Delete(context.Background(), route)
			Expect(err).ToNot(HaveOccurred())
			err = tconfig.DataPlaneClient(0).Delete(context.Background(), ns)
			Expect(err).ToNot(HaveOccurred())
		})

		FIt("sets the 'Programmed' condition to true", func() {
			gw := &gatewayapi.Gateway{}
			Eventually(func() bool {
				if err := tconfig.ControlPlaneClient().Get(ctx, types.NamespacedName{Name: gwname, Namespace: tconfig.TenantNamespace()}, gw); err != nil {
					return false
				}
				return meta.IsStatusConditionTrue(gw.Status.Conditions, string(gatewayapi.GatewayConditionProgrammed))
			}, 600*time.Second, 5*time.Second).Should(BeTrue())
		})

		It("makes available a hostname that resolves to the dataplate Gateway", func() {
			// TODO
		})

		It("makes available a hostname that is reachable by https", func() {
			// TODO
		})
	})

})
