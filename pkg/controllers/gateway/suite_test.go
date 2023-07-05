//go:build integration

/*
Copyright 2023 The MultiCluster Traffic Controller Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gateway

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ocmclusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	ocmworkv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/placement"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/tls"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

const (
	EventuallyTimeoutMedium   = time.Second * 10
	ConsistentlyTimeoutMedium = time.Second * 60
	TestRetryIntervalMedium   = time.Millisecond * 250
	nsSpoke1Name              = "test-spoke-cluster-1"
	nsSpoke2Name              = "test-spoke-cluster-2"
	defaultNS                 = "default"
	gatewayFinalizer          = "kuadrant.io/gateway"
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

	err = ocmworkv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ocmclusterv1beta1.AddToScheme(scheme.Scheme)
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

	err = (&GatewayClassReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	certificates := tls.NewService(k8sManager.GetClient(), "glbc-ca")

	dns := dns.NewService(k8sManager.GetClient(), dns.NewSafeHostResolver(dns.NewDefaultHostResolver()))

	plc := placement.NewOCMPlacer(k8sManager.GetClient())

	err = (&GatewayReconciler{
		Client:       k8sManager.GetClient(),
		Scheme:       k8sManager.GetScheme(),
		Certificates: certificates,
		Host:         dns,
		Placement:    plc,
	}).SetupWithManager(k8sManager, ctx)
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

var _ = Describe("GatewayClassController", func() {
	Context("testing gatewayclass controller", func() {
		var gatewayclass *gatewayv1beta1.GatewayClass
		BeforeEach(func() {
			gatewayclass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mgc-gw-controller",
				},
			}
		})

		AfterEach(func() {
			// Clean up GatewayClasses
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should accept a gatewayclass for this controller", func() {
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Status Accepted
			var condition metav1.Condition
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil && createdGatewayclass.Status.Conditions[0].Status == metav1.ConditionTrue
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			condition = createdGatewayclass.Status.Conditions[0]
			Expect(len(createdGatewayclass.Status.Conditions)).To(BeEquivalentTo(1))
			Expect(condition.Type).To(BeEquivalentTo(gatewayv1beta1.GatewayClassConditionStatusAccepted))
			Expect(condition.Reason).To(BeEquivalentTo(gatewayv1beta1.GatewayClassConditionStatusAccepted))
		})

		It("should NOT accept a gatewayclass for a different controller", func() {
			gatewayclass.Name = "some-other-gatewayclass"
			gatewayclass.Spec.ControllerName = "example.com/some-other-controller"
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Only 1
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(gatewayclassList.Items)).To(BeEquivalentTo(1))

			// Status stays Unknown
			var condition metav1.Condition
			Consistently(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				condition = createdGatewayclass.Status.Conditions[0]
				return condition.Type == string(gatewayv1beta1.GatewayClassConditionStatusAccepted) && condition.Status == metav1.ConditionUnknown
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should NOT accept a gatewayclass that is 'unsupported'", func() {
			gatewayclass.Name = "test-class-name-1"
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Status is false
			var condition metav1.Condition
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				condition = createdGatewayclass.Status.Conditions[0]
				return condition.Type == string(gatewayv1beta1.GatewayClassConditionStatusAccepted) && condition.Status == metav1.ConditionFalse
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(condition.Reason).To(BeEquivalentTo(gatewayv1beta1.GatewayClassReasonInvalidParameters))
			Expect(condition.Message).To(BeEquivalentTo("Invalid Parameters - Unsupported class name test-class-name-1. Must be one of [kuadrant-multi-cluster-gateway-instance-per-cluster]"))
		})
	})
})

var _ = Describe("GatewayController", func() {
	Context("testing gateway controller", func() {
		var gateway *gatewayv1beta1.Gateway
		var noLGateway *gatewayv1beta1.Gateway
		var gatewayClass *gatewayv1beta1.GatewayClass
		var managedZone *v1alpha1.ManagedZone
		var manifest1 *ocmworkv1.ManifestWork
		var manifest2 *ocmworkv1.ManifestWork
		var nsSpoke1 *corev1.Namespace
		var nsSpoke2 *corev1.Namespace
		var placementDecision *ocmclusterv1beta1.PlacementDecision
		var tlsSecrets *corev1.Secret
		var hostnametest gatewayv1beta1.Hostname
		var stringified string

		//Kicks off before the tests run, used to create resources we need in multiple tests,were created in a diff function that we are not testing but need the output, or things a user may have to do.
		BeforeEach(func() {
			// Before: Create GatewayClass for the gateway
			gatewayClass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Namespace: defaultNS,
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mctc-gw-controller",
				},
			}
			Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())

			// Before: Create a test ManagedZone for test Gateway listeners to use
			managedZone = &v1alpha1.ManagedZone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example.com",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.ManagedZoneSpec{
					ID:          "1234",
					DomainName:  "example.com",
					Description: "example.com",
				},
			}

			Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())

			//Before: Create placement decision

			placementDecision = &ocmclusterv1beta1.PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-placement",
					Namespace: defaultNS,
					Labels: map[string]string{
						"cluster.open-cluster-management.io/placement": "GatewayControllerTest",
					},
				},
			}

			Expect(k8sClient.Create(ctx, placementDecision)).To(BeNil())

			// Before: Update the statsus of theplacement decision to include the clusters we want to place as OCM would usually do this
			placementDecision.Status = ocmclusterv1beta1.PlacementDecisionStatus{
				Decisions: []ocmclusterv1beta1.ClusterDecision{
					{
						ClusterName: nsSpoke1Name,
						Reason:      "test",
					},
					{
						ClusterName: nsSpoke2Name,
						Reason:      "test",
					},
				},
			}
			Expect(k8sClient.Status().Update(ctx, placementDecision)).To(BeNil())
			//Before: Stub Gateway for tests but not creating it here
			hostname := gatewayv1beta1.Hostname("test.example.com")
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-1",
					Namespace: defaultNS,
					Labels: map[string]string{
						"cluster.open-cluster-management.io/placement": "GatewayControllerTest",
					},
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "default",
							Port:     8443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
							Hostname: &hostname,
						},
					},
				},
			}

			noLGateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-2",
					Namespace: defaultNS,
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "default",
							Port:     8443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
							Hostname: &hostname,
						},
					},
				},
			}
			// Before: Create namespace 1 to mock a spoke cluster
			nsSpoke1 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsSpoke1Name,
				},
			}
			err := k8sClient.Create(ctx, nsSpoke1)
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())

			}
			//Before: Create namespace 2 to mock a 2nd spoke cluster
			nsSpoke2 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsSpoke2Name,
				},
			}

			err = k8sClient.Create(ctx, nsSpoke2)
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())
			}
			// Before: Stub for a tls secret but dont create it until further down
			tlsSecrets = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.example.com",
					Namespace: "default",
				},
				StringData: map[string]string{
					"tls.key": "some_value",
				},
			}

		})
		// Occurs after the test is complete
		AfterEach(func() {
			// Clean: Gateways
			gatewayList := &gatewayv1beta1.GatewayList{}
			err := k8sClient.List(ctx, gatewayList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gateway := range gatewayList.Items {
				err = k8sClient.Delete(ctx, &gateway)
				Expect(err).NotTo(HaveOccurred())
			}

			// Clean: GatewayClasses
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err = k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}

			// Clean: ManagedZones
			managedZoneList := &v1alpha1.ManagedZoneList{}
			err = k8sClient.List(ctx, managedZoneList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, managedZone := range managedZoneList.Items {
				err = k8sClient.Delete(ctx, &managedZone)
				Expect(err).NotTo(HaveOccurred())
			}
			// Clean: Placement decisions
			placementDecisionList := &ocmclusterv1beta1.PlacementDecisionList{}
			err = k8sClient.List(ctx, placementDecisionList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, placementDecision := range placementDecisionList.Items {
				err = k8sClient.Delete(ctx, &placementDecision)
				Expect(err).NotTo(HaveOccurred())
			}

		})

		// This tests the full reconcile of a gateway from start to finish
		It("should reconcile a gateway", func() {
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			upstreamGateway := &gatewayv1beta1.Gateway{}
			upstreamGatewayType := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}
			manifest1 = &ocmworkv1.ManifestWork{}
			manifest2 = &ocmworkv1.ManifestWork{}

			// Test: Passes if it gets the gateway
			Eventually(func() bool {
				err := k8sClient.Get(ctx, upstreamGatewayType, upstreamGateway)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Test: Passes if the gateway contains a finalizer
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, upstreamGatewayType, upstreamGateway); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(upstreamGateway, gatewayFinalizer)
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Test: Passes if the gateway has the correct label
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, upstreamGatewayType, gateway); err != nil {
					return false
				}
				return gateway.Labels["kuadarant.io/managed"] == "true"

			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Mock: Creating tls cert manager would have created
			err := k8sClient.Create(ctx, tlsSecrets)
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())

			}

			// Test: Passes when manifest1 is found in Namespace test-spoke-cluster-1 and contains the hostname from the gateway
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nsSpoke1Name, Name: "gateway-default-test-gw-1"}, manifest1); err != nil {
					log.Log.Error(err, "error getting ManifestWork")

					return false
				}
				// decoding the raw format in the manifest 1 into a readable variable that can be compared
				rawBytes := manifest1.Spec.Workload.Manifests[0].Raw
				gateway := &gatewayv1beta1.Gateway{}
				err := json.Unmarshal(rawBytes, gateway)
				if err != nil {
					return false
				}
				hostnametest = *gateway.Spec.Listeners[0].Hostname
				stringified = string(hostnametest)
				return err == nil

			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			//Comparing the hostname
			Expect(stringified).To(Equal("test.example.com"))

			// Test: Passes when manifest2 is found in Namespace test-spoke-cluster-2 and contains the hostname from the gateway

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nsSpoke2Name, Name: "gateway-default-test-gw-1"}, manifest2); err != nil {
					log.Log.Error(err, "error getting ManifestWork")
					return false
				}
				// decoding the raw format in the manifest 1 into a readable variable that can be compared

				rawBytes := manifest2.Spec.Workload.Manifests[0].Raw
				gateway := &gatewayv1beta1.Gateway{}
				err := json.Unmarshal(rawBytes, gateway)
				if err != nil {
					return false
				}
				hostnametest = *gateway.Spec.Listeners[0].Hostname
				stringified = string(hostnametest)
				return err == nil

			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			//Comparing the hostname
			Expect(stringified).To(Equal("test.example.com"))

			//Mock: Make the manifestwork status in manifest 1 be "applied". OCM usually does it when the resources have been applied
			manifest1.Status = ocmworkv1.ManifestWorkStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Applied",
						Status:             "True",
						LastTransitionTime: metav1.Now(),
						Reason:             "AppliedManifestComplete",
						Message:            "Apply manifest complete",
					},
				},
			}

			Expect(k8sClient.Status().Update(ctx, manifest1)).To(BeNil())

			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: manifest2.Namespace, Name: manifest2.Name}, manifest2)).To(BeNil())
			//Mock: Make the manifestwork status in manifest 2 be "applied". OCM usually does it when the resources have been applied
			manifest2.Status = ocmworkv1.ManifestWorkStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Applied",
						Status:             "True",
						LastTransitionTime: metav1.Now(),
						Reason:             "AppliedManifestComplete",
						Message:            "Apply manifest complete",
					},
				},
			}

			Expect(k8sClient.Status().Update(ctx, manifest2)).To(BeNil())

			// Test that the Programmed condition is True
			var programmedCondition *metav1.Condition
			Eventually(func() bool {
				err := k8sClient.Get(ctx, upstreamGatewayType, upstreamGateway)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				programmedCondition = meta.FindStatusCondition(upstreamGateway.Status.Conditions, string(gatewayv1beta1.GatewayConditionProgrammed))
				log.Log.Info("programmedCondition", "programmedCondition", programmedCondition)
				Expect(programmedCondition).ToNot(BeNil())
				return programmedCondition.Status == metav1.ConditionTrue
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		})

		// Tests if the placement label isnt present no manifest should be created
		It("shouldnt place a gateway", func() {

			Expect(k8sClient.Create(ctx, noLGateway)).To(BeNil())
			noLabelGateway := &gatewayv1beta1.Gateway{}
			noLabelUpstreamGatewayType := types.NamespacedName{Name: noLGateway.Name, Namespace: noLGateway.Namespace}

			// Passes if it gets the gateway
			Eventually(func() bool {
				err := k8sClient.Get(ctx, noLabelUpstreamGatewayType, noLabelGateway)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Passes if the gateway contains a finalizer
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, noLabelUpstreamGatewayType, noLabelGateway); err != nil {
					return false
				}
				return controllerutil.ContainsFinalizer(noLabelGateway, gatewayFinalizer)
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Test: Passes if no manifest was gotten
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nsSpoke1Name, Name: "gateway-default-test-gw-2"}, manifest1)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeFalse())

		})
	})
	Context("testing gateway controller access to managed zones", func() {
		var gateway *gatewayv1beta1.Gateway
		var gatewayClass *gatewayv1beta1.GatewayClass
		var managedZone *v1alpha1.ManagedZone
		// var cluster1 *corev1.Secret
		BeforeEach(func() {
			// Create a test GatewayClass for test Gateways to reference
			gatewayClass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mgc-gw-controller",
				},
			}
			// Create a test ManagedZone for test Gateway listeners to use
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
					Name:      "test-gw-1",
					Namespace: "default",
					Annotations: map[string]string{
						"kuadrant.io/gateway-cluster-label-selector": "type=test",
					},
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "default",
							Port:     8443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
							Hostname: &hostname,
						},
					},
				},
			}
		})
		AfterEach(func() {
			// Clean up Gateways
			gatewayList := &gatewayv1beta1.GatewayList{}
			err := k8sClient.List(ctx, gatewayList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gateway := range gatewayList.Items {
				err = k8sClient.Delete(ctx, &gateway)
				Expect(err).NotTo(HaveOccurred())
			}
			// Clean up GatewayClasses
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err = k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}
			// Clean up ManagedZones
			managedZoneList := &v1alpha1.ManagedZoneList{}
			err = k8sClient.List(ctx, managedZoneList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, managedZone := range managedZoneList.Items {
				err = k8sClient.Delete(ctx, &managedZone)
				Expect(err).NotTo(HaveOccurred())
			}
		})
		It("should accept a managed zone and permit related hosts and deny unrelated hosts", func() {
			managedZone.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())

			createdMZ := &v1alpha1.ManagedZone{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: managedZone.Namespace, Name: managedZone.Name}, createdMZ)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			createdGW := &gatewayv1beta1.Gateway{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, createdGW)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, createdGW)
				if err != nil {
					return false
				}

				gwIsProgrammed := false
				for _, condition := range createdGW.Status.Conditions {
					if condition.Type == "Programmed" && condition.Status == metav1.ConditionTrue {
						gwIsProgrammed = true
					}
				}

				return gwIsProgrammed
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, createdGW)).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: createdGW.Namespace, Name: createdGW.Name}, createdGW)
				return k8serrors.IsNotFound(err)
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			badHost := gatewayv1beta1.Hostname("test.badexample.com")
			gateway.ResourceVersion = ""
			gateway.Spec.Listeners[0].Hostname = &badHost

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, createdGW)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, createdGW)
				if err != nil {
					return false
				}

				gwIsProgrammed := false
				for _, condition := range createdGW.Status.Conditions {
					if condition.Type == "Programmed" && condition.Status == metav1.ConditionTrue {
						gwIsProgrammed = true
					}
				}

				return !gwIsProgrammed
			}, ConsistentlyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})
		It("should accept a managed zone and ignore it for gateways in other namespaces", func() {
			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-2",
				},
			}
			Expect(k8sClient.Create(ctx, &ns)).To(BeNil())

			managedZone.Namespace = "test-2"
			Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())

			createdMZ := &v1alpha1.ManagedZone{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: managedZone.Namespace, Name: managedZone.Name}, createdMZ)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
			createdGWClass := &gatewayv1beta1.GatewayClass{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, createdGWClass)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			createdGW := &gatewayv1beta1.Gateway{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, createdGW)
				return err == nil
			}, EventuallyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, createdGW)
				if err != nil {
					return false
				}

				gwIsProgrammed := false
				for _, condition := range createdGW.Status.Conditions {
					if condition.Type == "Programmed" && condition.Status == metav1.ConditionTrue {
						gwIsProgrammed = true
					}
				}

				return !gwIsProgrammed
			}, ConsistentlyTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})
	})
})
