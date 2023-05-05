/*
Copyright 2022 The MultiCluster Traffic Controller Authors.

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

package ratelimitpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	kuadrantv1beta1 "github.com/kuadrant/kuadrant-operator/api/v1beta1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/clusterSecret"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/metadata"
	cpcv1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/config-policy-controller/api/v1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/placement"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer/mutator"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	gppv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
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
			filepath.Join("../../../", "config", "ocm", "api", "cluster", "v1beta1"),
			filepath.Join("../../../", "config", "ocm", "gpp", "deploy", "crds"),
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

	err = kuadrantv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = clusterv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = gppv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = cpcv1.AddToScheme(scheme.Scheme)
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

	placement, err := placement.NewOCMPlacer(k8sManager.GetConfig(), k8sManager.GetClient())
	clusterSecretService := clusterSecret.NewService(k8sManager.GetClient())
	err = (&RateLimitPolicyReconciler{
		Client:         k8sManager.GetClient(),
		Scheme:         k8sManager.GetScheme(),
		ClusterSecrets: clusterSecretService,
		Placement:      placement,
	}).SetupWithManager(k8sManager)
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

var _ = Describe("RateLimitPolicyController", func() {
	var clusterSecret1 *corev1.Secret
	var clusterSecret2 *corev1.Secret
	var gateway *gatewayv1beta1.Gateway
	var gatewayclass *gatewayv1beta1.GatewayClass
	var placementDecision *clusterv1beta1.PlacementDecision
	var rlp *kuadrantv1beta1.RateLimitPolicy
	var ns1 *corev1.Namespace
	var ns2 *corev1.Namespace

	BeforeEach(func() {

		// Create test cluster secrets
		clusterSecret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-1-secret",
				Namespace: "default",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
				},
			},
			StringData: map[string]string{
				"name":   "test-cluster-1",
				"config": "{}",
			},
		}
		Expect(k8sClient.Create(ctx, clusterSecret1)).To(BeNil())

		clusterSecret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-2-secret",
				Namespace: "default",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
				},
			},
			StringData: map[string]string{
				"name":   "test-cluster-2",
				"config": "{}",
			},
		}
		Expect(k8sClient.Create(ctx, clusterSecret2)).To(BeNil())

		// Create a test GatewayClass for test Gateways to reference
		gatewayclass = &gatewayv1beta1.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mctc-gw-istio-external-instance-per-cluster",
				Namespace: "default",
			},
			Spec: gatewayv1beta1.GatewayClassSpec{
				ControllerName: "kuadrant.io/mctc-gw-controller",
			},
		}
		Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())

		// Create a test PlacementDecision for test Gateways to reference
		placementDecision = &clusterv1beta1.PlacementDecision{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-placement-1",
				Namespace: "default",
				Labels: map[string]string{
					placement.OCMPlacementLabel: "test-placement",
				},
			},
		}
		Expect(k8sClient.Create(ctx, placementDecision)).To(BeNil())

		// Updatde the status with decisions
		placementDecision.Status = clusterv1beta1.PlacementDecisionStatus{
			Decisions: []clusterv1beta1.ClusterDecision{
				{
					ClusterName: "test-cluster-1",
					Reason:      "test",
				},
				{
					ClusterName: "test-cluster-2",
					Reason:      "test",
				},
			},
		}
		Expect(k8sClient.Status().Update(ctx, placementDecision)).To(BeNil())

		// Create the placement namespaces
		ns1 = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-1",
			},
		}
		ns2 = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-2",
			},
		}
		// TODO: Revisit this logic that ignores namespaces already existing.
		// It was implemented this way because of errors when trying to clean
		// up namespaces. Each time the namespace wouldn't be deleted in a timely
		// manner, leading to the below error on subsequent test runs:
		//
		// object is being deleted: namespaces \"test-cluster-1\" already exists
		err := k8sClient.Create(ctx, ns1)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Expect(err).ToNot(HaveOccurred())
		}
		err = k8sClient.Create(ctx, ns2)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			Expect(err).ToNot(HaveOccurred())
		}

		rlp = &kuadrantv1beta1.RateLimitPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rlp-1",
				Namespace: "default",
			},
			Spec: kuadrantv1beta1.RateLimitPolicySpec{
				TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
					Group: "gateway.networking.k8s.io",
					Kind:  "Gateway",
					Name:  "test-gw-1",
				},
			},
		}

	})

	AfterEach(func() {

		// Clean up RateLimitPolicies
		rlpList := &kuadrantv1beta1.RateLimitPolicyList{}
		err := k8sClient.List(ctx, rlpList, client.InNamespace("default"))
		Expect(err).NotTo(HaveOccurred())
		for _, rlp := range rlpList.Items {
			err = k8sClient.Delete(ctx, &rlp)
			Expect(err).NotTo(HaveOccurred())
		}

		// Clean up Gateways
		gatewayList := &gatewayv1beta1.GatewayList{}
		err = k8sClient.List(ctx, gatewayList, client.InNamespace("default"))
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

		// Clean up Clusters
		clusterList := &corev1.SecretList{}
		listOptions := client.MatchingLabels{
			"argocd.argoproj.io/secret-type": "cluster",
		}
		err = k8sClient.List(ctx, clusterList, listOptions, client.InNamespace("default"))
		Expect(err).NotTo(HaveOccurred())
		for _, cluster := range clusterList.Items {
			err = k8sClient.Delete(ctx, &cluster)
			Expect(err).NotTo(HaveOccurred())
		}

		// Clean up PlacementDecisions
		placementDecisionList := &clusterv1beta1.PlacementDecisionList{}
		err = k8sClient.List(ctx, placementDecisionList, client.InNamespace("default"))
		Expect(err).NotTo(HaveOccurred())
		for _, placementDecision := range placementDecisionList.Items {
			err = k8sClient.Delete(ctx, &placementDecision)
			Expect(err).NotTo(HaveOccurred())
		}

		// Clean up Policies
		policyList := &gppv1.PolicyList{}
		err = k8sClient.List(ctx, policyList, client.InNamespace("test-cluster-1"))
		Expect(err).NotTo(HaveOccurred())
		for _, policy := range policyList.Items {
			err = k8sClient.Delete(ctx, &policy)
			Expect(err).NotTo(HaveOccurred())
		}
		err = k8sClient.List(ctx, policyList, client.InNamespace("test-cluster-2"))
		Expect(err).NotTo(HaveOccurred())
		for _, policy := range policyList.Items {
			err = k8sClient.Delete(ctx, &policy)
			Expect(err).NotTo(HaveOccurred())
		}

	})

	It("should reconcile a rate limit policy", func() {

		// Create a test Gateway for test RateLimitPolicies to reference
		hostname := gatewayv1beta1.Hostname("test.example.com")
		gateway = &gatewayv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-gw-1",
				Namespace: "default",
				Labels: map[string]string{
					placement.OCMPlacementLabel: "test-placement",
				},
			},
			Spec: gatewayv1beta1.GatewaySpec{
				GatewayClassName: "mctc-gw-istio-external-instance-per-cluster",
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
		Expect(k8sClient.Create(ctx, gateway)).To(BeNil())

		Expect(k8sClient.Create(ctx, rlp)).To(BeNil())
		createdRlp := &kuadrantv1beta1.RateLimitPolicy{}
		rlpType := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}

		// Exists
		Eventually(func() bool {
			err := k8sClient.Get(ctx, rlpType, createdRlp)
			return err == nil
		}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		// Check Finalizer set
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(createdRlp, RateLimitPolicyFinalizer)
		}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		// Check Gateway Owner set
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
				return false
			}
			return metav1.IsControlledBy(createdRlp, gateway)
		}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
	})

	Context("sync all clusters", func() {
		var createdRlp *kuadrantv1beta1.RateLimitPolicy

		BeforeEach(func() {

			// Create a test Gateway for test RateLimitPolicies to reference
			// Gateway set to sync to `all` clusters
			hostname := gatewayv1beta1.Hostname("test.example.com")
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-1",
					Namespace: "default",
					Annotations: map[string]string{
						"kuadrant.io/gateway-cluster-label-selector": "type=test",
						"mctc-sync-agent/all":                        "true",
					},
					Labels: map[string]string{
						placement.OCMPlacementLabel: "test-placement",
					},
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "mctc-gw-istio-external-instance-per-cluster",
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
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())

			Expect(k8sClient.Create(ctx, rlp)).To(BeNil())
			createdRlp = &kuadrantv1beta1.RateLimitPolicy{}
			rlpType := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		})

		It("should create an OCM Policy in all cluster namespaces", func() {
			Eventually(func() bool {
				policyTypeCluster1 := types.NamespacedName{Name: rlp.Name, Namespace: "test-cluster-1"}
				policyTypeCluster2 := types.NamespacedName{Name: rlp.Name, Namespace: "test-cluster-2"}
				createdPolicy := &gppv1.Policy{}
				// Check cluster 1 namespace
				if err := k8sClient.Get(ctx, policyTypeCluster1, createdPolicy); err != nil {
					return false
				}
				// Check cluster 2 namespace
				if err := k8sClient.Get(ctx, policyTypeCluster2, createdPolicy); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		})

		XIt("should apply a descriptor key & value for all clusters", func() {
			// Check patch annotation set for all cluster
			Eventually(func() bool {
				policyTypeCluster1 := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}
				// policyTypeCluster2 := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}
				createdPolicy := &gppv1.Policy{}

				// Check cluster 1 namespace
				if err := k8sClient.Get(ctx, policyTypeCluster1, createdPolicy); err != nil {
					return false
				}
				// parse out the RLP from the ConfigurationPolicy in the Policy
				var configPolicy *cpcv1.ConfigurationPolicy
				configPolicyObjDef := createdPolicy.Spec.PolicyTemplates[0].ObjectDefinition
				Expect(json.Unmarshal(configPolicyObjDef.Raw, configPolicy)).NotTo(HaveOccurred())
				var rlp *kuadrantv1beta1.RateLimitPolicy
				rlpObjDef := configPolicy.Spec.ObjectTemplates[0].ObjectDefinition
				Expect(json.Unmarshal(rlpObjDef.Raw, rlp)).NotTo(HaveOccurred())

				rateLimit := rlp.Spec.RateLimits[0]
				action := rateLimit.Configurations[0].Actions[0]
				if *action.GenericKey.DescriptorKey != "kuadrant_gateway_cluster" {
					return false
				}
				descriptorValue := action.GenericKey.DescriptorValue
				if descriptorValue != "test-cluster-1" {
					return false
				}

				// Check cluster 2 namespace
				// if err := k8sClient.Get(ctx, policyTypeCluster2, createdPolicy); err != nil {
				// 	return false
				// }
				// // parse out the RLP
				// rlp = &kuadrantv1beta1.RateLimitPolicy{}
				// rateLimit = rlp.Spec.RateLimits[0]
				// action = rateLimit.Configurations[0].Actions[0]
				// if *action.GenericKey.DescriptorKey != "kuadrant_gateway_cluster" {
				// 	return false
				// }
				// descriptorValue = action.GenericKey.DescriptorValue
				// if descriptorValue != "test-cluster-2" {
				// 	return false
				// }
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		})
	})

	Context("sync explicit cluster", func() {
		var createdRlp *kuadrantv1beta1.RateLimitPolicy

		BeforeEach(func() {

			// Create a test Gateway for test RateLimitPolicies to reference
			// Gateway set to sync to `test-cluster-1` cluster
			hostname := gatewayv1beta1.Hostname("test.example.com")
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-1",
					Namespace: "default",
					Annotations: map[string]string{
						"kuadrant.io/gateway-cluster-label-selector": "type=test",
						"mctc-sync-agent/test-cluster-1":             "true",
					},
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "mctc-gw-istio-external-instance-per-cluster",
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
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())

			Expect(k8sClient.Create(ctx, rlp)).To(BeNil())
			createdRlp = &kuadrantv1beta1.RateLimitPolicy{}
			rlpType := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		XIt("should add sync annotations for test-cluster-1", func() {
			// Check sync annotation set for test-cluster-1
			Eventually(func() bool {
				rlpType := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}
				if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
					return false
				}
				// Check cluster 1
				annotationKey := fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "test-cluster-1")
				return metadata.HasAnnotation(createdRlp, annotationKey)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		XIt("should add patch annotations for test-cluster-1", func() {
			// Check patch annotation set for test-cluster-1
			Eventually(func() bool {
				rlpType := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}
				if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
					return false
				}
				// Check cluster 1
				annotationKey := fmt.Sprintf("%s%s", mutator.JSONPatchAnnotationPrefix, "test-cluster-1")
				if hasAnnotation := metadata.HasAnnotation(createdRlp, annotationKey); !hasAnnotation {
					return false
				}
				annotationValue := metadata.GetAnnotation(createdRlp, annotationKey)
				expectedValue := "[{\"op\":\"add\",\"path\":\"/spec/rateLimits\",\"value\":[{\"configurations\":[{\"actions\":[{\"generic_key\":{\"descriptor_key\":\"kuadrant_gateway_cluster\",\"descriptor_value\":\"test-cluster-1\"}}]}]}]}]"
				if annotationValue != expectedValue {
					return false
				}
				return annotationValue == expectedValue
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

		})

		XIt("should not add sync annotations for test-cluster-2", func() {
			Consistently(func() bool {
				rlpType := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}
				if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
					return false
				}
				annotationKey := fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "test-cluster-2")
				return !metadata.HasAnnotation(createdRlp, annotationKey)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		XIt("should not add patch annotations for test-cluster-2", func() {
			Consistently(func() bool {
				rlpType := types.NamespacedName{Name: rlp.Name, Namespace: rlp.Namespace}
				if err := k8sClient.Get(ctx, rlpType, createdRlp); err != nil {
					return false
				}
				annotationKey := fmt.Sprintf("%s%s", mutator.JSONPatchAnnotationPrefix, "test-cluster-2")
				return !metadata.HasAnnotation(createdRlp, annotationKey)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

	})
})
