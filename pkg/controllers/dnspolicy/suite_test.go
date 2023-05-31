package dnspolicy

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	kuadrantapi "github.com/kuadrant/kuadrant-operator/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/placement"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/tls"
	//+kubebuilder:scaffold:imports
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

const (
	TestTimeoutMedium       = time.Second * 10
	TestRetryIntervalMedium = time.Millisecond * 250
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../../", "config", "crd", "bases"),
			filepath.Join("../../../", "config", "gateway-api", "crd", "standard"),
			filepath.Join("../../../", "config", "cert-manager", "crd", "v1.7.1"),
			filepath.Join("../../../", "config", "kuadrant", "crd"),
			filepath.Join("../../../", "config", "ocm", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = gatewayv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = certman.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = kuadrantapi.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = workv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = clusterv1beta2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		HealthProbeBindAddress: "0",
		MetricsBindAddress:     "0",
	})
	Expect(err).ToNot(HaveOccurred())

	dnsProvider := &dns.FakeProvider{}
	dns := dns.NewService(k8sManager.GetClient(), dns.NewSafeHostResolver(dns.NewDefaultHostResolver()), dnsProvider)
	err = (&DNSPolicyReconciler{
		Client:      k8sManager.GetClient(),
		DNSProvider: dnsProvider,
		HostService: dns,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&gateway.GatewayClassReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	certificates := tls.NewService(k8sManager.GetClient(), "glbc-ca")
	plc, err := placement.NewOCMPlacer(k8sManager.GetConfig())
	Expect(err).ToNot(HaveOccurred())
	err = (&gateway.GatewayReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		Certificates: certificates,
		Host:         dns,
		Placement:    plc,
	}).SetupWithManager(k8sManager, ctx)
	Expect(err).ToNot(HaveOccurred())

	// TODO: can we avoid duplicate set code here that also in controller/main.go?
	err = k8sManager.GetFieldIndexer().IndexField(
		context.Background(),
		&v1alpha1.ManagedZone{},
		"spec.domainName",
		func(obj client.Object) []string {
			return []string{obj.(*v1alpha1.ManagedZone).Spec.DomainName}
		},
	)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("DNSPolicy", func() {
	Context("testing DNSPolicy controller", func() {
		var dnsPolicy *v1alpha1.DNSPolicy
		var gatewayClass *gatewayv1beta1.GatewayClass
		var managedZone *v1alpha1.ManagedZone
		var gateway *gatewayv1beta1.Gateway
		var dnsRecord *v1alpha1.DNSRecord

		protocol := v1alpha1.HttpProtocol
		healthCheckSpec := v1alpha1.HealthCheckSpec{
			Endpoint: "/",
			Protocol: &protocol,
		}
		BeforeEach(func() {
			ns := gatewayapiv1alpha2.Namespace("default")
			dnsPolicy = &v1alpha1.DNSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dns-policy",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSPolicySpec{
					TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
						Group:     "",
						Kind:      "Gateway",
						Name:      "example-gateway",
						Namespace: &ns,
					},
					HealthCheck: &healthCheckSpec,
				},
			}

			gatewayClass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mctc-gw-controller",
				},
			}
			managedZone = &v1alpha1.ManagedZone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example.com",
					Namespace: "default",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					ID:          "1234",
					DomainName:  "example.com",
					Description: "example.com",
				},
			}
			hostname := gatewayv1beta1.Hostname("test.example.com")
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-gateway",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "test.example.com",
							Hostname: &hostname,
							Port:     8080,
							Protocol: gatewayv1beta1.HTTPProtocolType,
						},
					},
				},
			}
			dnsRecord = &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.example.com",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSRecordSpec{
					ManagedZoneRef: &v1alpha1.ManagedZoneReference{
						Name: managedZone.Name,
					},
					Endpoints: []*v1alpha1.Endpoint{
						{
							DNSName:       "test.example.com",
							SetIdentifier: "test.example.com",
							Targets:       v1alpha1.Targets{"127.0.0.1"},
							RecordType:    string(v1alpha1.ARecordType),
							RecordTTL:     60,
						},
					},
				},
				Status: v1alpha1.DNSRecordStatus{},
			}
		})

		AfterEach(func() {
			// Clean up
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}

		})

		It("should reconcile a DNS Policy", func() {
			Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
			Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())
			createdGatewayClass := &gatewayv1beta1.GatewayClass{}
			createdMZ := &v1alpha1.ManagedZone{}

			Eventually(func() bool { // gateway class exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, createdGatewayClass); err != nil {
					return false
				}
				return true

			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Eventually(func() bool { // managed zone exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: managedZone.Name, Namespace: managedZone.Namespace}, createdMZ); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
			createdDNSPolicy := &v1alpha1.DNSPolicy{}
			Eventually(func() bool { //dns policy exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, createdDNSPolicy); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			//no health check status should be present yet
			Expect(dnsPolicy.Status.HealthCheck).To(BeNil())

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			createdGW := &gatewayv1beta1.Gateway{}
			Eventually(func() bool { //gateway exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, createdGW); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Eventually(func() bool { //dns policy exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, createdDNSPolicy); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			//no health check status should be present yet
			Expect(dnsPolicy.Status.HealthCheck).To(BeNil())

			metadata.AddLabel(dnsRecord, dns.LabelGatewayReference, string(createdGW.UID))
			Expect(k8sClient.Create(ctx, dnsRecord)).To(BeNil())
			createdDNSRecord := &v1alpha1.DNSRecord{}
			Eventually(func() bool { // DNS record exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsRecord.Name, Namespace: dnsRecord.Namespace}, createdDNSRecord); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Eventually(func() bool { // DNS Policy has health check status
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, createdDNSPolicy); err != nil {
					return false
				}

				if createdDNSPolicy.Status.HealthCheck == nil || createdDNSPolicy.Status.HealthCheck.Conditions == nil {
					return false
				}
				return len(createdDNSPolicy.Status.HealthCheck.Conditions) > 0
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})
	})
})
