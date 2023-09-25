//go:build e2e

package e2e

import (
	"strings"
	"time"

	v1 "github.com/jetstack/cert-manager/pkg/apis/acme/v1"
	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	testUtil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("TLSPolicy", func() {

	// testID is a randomly generated identifier for the test
	// it is used to name resources and/or namespaces so different
	// tests can be run in parallel in the same cluster
	var testID string
	var hostname gatewayapi.Hostname
	var gw *gatewayapi.Gateway
	var issuer *certmanv1.Issuer
	var managedZone *v1alpha1.ManagedZone
	var issuerSecret *corev1.Secret
	var tlsPolicy *v1alpha1.TLSPolicy

	BeforeEach(func(ctx SpecContext) {
		testID = "t-e2e-" + tconfig.GenerateName()
		By("Creating a gateway")
		hostname = gatewayapi.Hostname(strings.Join([]string{testID, tconfig.ManagedZone()}, "."))
		gw = &gatewayapi.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID,
				Namespace: tconfig.HubNamespace(),
				Labels:    map[string]string{"gw": "t-e2e"},
			},
			Spec: gatewayapi.GatewaySpec{
				GatewayClassName: testUtil.GatewayClassName,
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
							From: testUtil.Pointer(gatewayapi.NamespacesFromAll),
						},
					},
				}},
			},
		}

		err := tconfig.HubClient().Create(ctx, gw)
		Expect(err).ToNot(HaveOccurred())

		By("Creating a issuer")
		// get the managed zone to get the credentials secret ref
		managedZone = &v1alpha1.ManagedZone{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tconfig.ManagedZone(),
				Namespace: tconfig.HubNamespace(),
			},
		}

		err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: managedZone.Name, Namespace: managedZone.Namespace}, managedZone)
		Expect(err).ToNot(HaveOccurred())
		// get the provider secret
		providerSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedZone.Spec.SecretRef.Name,
				Namespace: managedZone.Spec.SecretRef.Namespace,
			}}
		err = tconfig.HubClient().Get(ctx, client.ObjectKey{Name: providerSecret.Name, Namespace: providerSecret.Namespace}, providerSecret)
		Expect(err).ToNot(HaveOccurred())

		// create the Let's encrypt credentials secret
		issuerSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "le-aws-credentials",
				Namespace: tconfig.HubNamespace(),
			},
			Data: map[string][]byte{
				"AWS_SECRET_ACCESS_KEY": providerSecret.Data["AWS_SECRET_ACCESS_KEY"],
			},
		}
		err = tconfig.HubClient().Create(ctx, issuerSecret)
		Expect(client.IgnoreAlreadyExists(err)).ToNot(HaveOccurred())

		issuer = &certmanv1.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "le-production",
				Namespace: tconfig.HubNamespace(),
			},
			Spec: certmanv1.IssuerSpec{
				IssuerConfig: certmanv1.IssuerConfig{
					ACME: &v1.ACMEIssuer{
						Email:          "kuadrant@redhat.com",
						Server:         "https://acme-v02.api.letsencrypt.org/directory",
						PreferredChain: "",
						PrivateKey: cmmeta.SecretKeySelector{
							LocalObjectReference: cmmeta.LocalObjectReference{Name: "le-production"},
						},
						Solvers: []v1.ACMEChallengeSolver{
							{
								DNS01: &v1.ACMEChallengeSolverDNS01{
									Route53: &v1.ACMEIssuerDNS01ProviderRoute53{
										AccessKeyID: string(providerSecret.Data["AWS_ACCESS_KEY_ID"]),
										SecretAccessKey: cmmeta.SecretKeySelector{
											LocalObjectReference: cmmeta.LocalObjectReference{Name: "le-aws-credentials"},
											Key:                  "AWS_SECRET_ACCESS_KEY",
										},
										HostedZoneID: managedZone.Spec.ID,
										Region:       "us-east-1",
									},
								},
							},
						},
					},
				},
			},
		}
		err = tconfig.HubClient().Create(ctx, issuer)
		Expect(err).ToNot(HaveOccurred())

		//Create the TLSPolicy
		tlsPolicy = &v1alpha1.TLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testID,
				Namespace: tconfig.HubNamespace(),
			},
			Spec: v1alpha1.TLSPolicySpec{
				TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
					Group:     "gateway.networking.k8s.io",
					Kind:      "Gateway",
					Name:      gatewayapi.ObjectName(testID),
					Namespace: testUtil.Pointer(gatewayapi.Namespace(tconfig.HubNamespace())),
				},
				CertificateSpec: v1alpha1.CertificateSpec{
					IssuerRef: cmmeta.ObjectReference{
						Name:  "le-production",
						Kind:  "Issuer",
						Group: "cert-manager.io",
					},
				},
			},
		}
		err = tconfig.HubClient().Create(ctx, tlsPolicy)
		Expect(err).ToNot(HaveOccurred())

	})

	AfterEach(func(ctx SpecContext) {
		err := tconfig.HubClient().Delete(ctx, gw,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		err = tconfig.HubClient().Delete(ctx, issuer,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		err = tconfig.HubClient().Delete(ctx, issuerSecret,
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		err = tconfig.HubClient().Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testID}},
			client.PropagationPolicy(metav1.DeletePropagationForeground))
		Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
	})

	When("issuer and gateway are created", func() {

		It("certificates are created", func(ctx SpecContext) {

			Eventually(func(ctx SpecContext) error {
				cl := &certmanv1.CertificateList{}
				err := tconfig.HubClient().List(ctx, cl, client.InNamespace(tconfig.HubNamespace()))
				Expect(err).ToNot(HaveOccurred())
				// TODO verify certs are as expected
				return nil
			}).WithContext(ctx).WithTimeout(10 * time.Second).WithPolling(2 * time.Second).ShouldNot(HaveOccurred())

			// TODO Do a check on tls secrets

			// TODO Validate the HTTPs via the browser or curl

		})
	})

})
