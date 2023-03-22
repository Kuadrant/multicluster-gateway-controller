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

package placement

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
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
	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("../../../", "config", "crd", "bases"),
			filepath.Join("../../../", "config", "gateway-api", "crd", "standard"),
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

	err = (&PlacementReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
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

var _ = Describe("PlacementController", func() {
	Context("testing placement controller", func() {
		var placement *v1alpha1.Placement
		var gateway *gatewayv1beta1.Gateway
		var cluster1 *corev1.Secret

		BeforeEach(func() {
			// Create a test GatewayClass for test Gateways to reference
			gatewayclass := &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mctc-gw-istio-external-instance-per-cluster",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mctc-gw-controller",
				},
			}
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())

			// Stub Gateway for tests
			hostname := gatewayv1beta1.Hostname("test.example.com")
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-1",
					Namespace: "default",
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

			// test cluster secrets
			cluster1 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster1",
					Namespace: "default",
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "cluster",
						"region":                         "us-east-1",
						"country":                        "us",
					},
				},
				StringData: map[string]string{
					"name":   "cluster1",
					"server": "https://cluster1-control-plane:6443",
					"config": "{\"tlsClientConfig\":{\"insecure\":true,\"caData\":\"test\",\"certData\":\"test\",\"keyData\":\"test\"}}",
				},
			}
			Expect(k8sClient.Create(ctx, cluster1)).To(BeNil())
			cluster2 := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster2",
					Namespace: "default",
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "cluster",
						"region":                         "us-west-1",
						"country":                        "us",
					},
				},
				StringData: map[string]string{
					"name":   "cluster2",
					"server": "https://cluster2-control-plane:6443",
					"config": "{\"tlsClientConfig\":{\"insecure\":true,\"caData\":\"test\",\"certData\":\"test\",\"keyData\":\"test\"}}",
				},
			}
			Expect(k8sClient.Create(ctx, cluster2)).To(BeNil())

			// test placement
			placement = &v1alpha1.Placement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-placement-1",
					Namespace: "default",
				},
				Spec: v1alpha1.PlacementSpec{
					TargetRef: v1alpha2.PolicyTargetReference{
						Group: "gateway.networking.k8s.io",
						Kind:  "Gateway",
						Name:  "test-gw-1",
					},
					Predicates: []v1alpha1.ClusterPredicate{
						{
							RequiredClusterSelector: v1alpha1.ClusterSelector{
								LabelSelector: metav1.LabelSelector{
									MatchLabels: map[string]string{
										"region": "us-east-1",
									},
								},
							},
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

			// Clean up Placements
			placementList := &v1alpha1.PlacementList{}
			err = k8sClient.List(ctx, placementList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, placement := range placementList.Items {
				err = k8sClient.Delete(ctx, &placement)
				Expect(err).NotTo(HaveOccurred())
			}

			// Clean up Cluster Secrets
			secretList := &corev1.SecretList{}
			err = k8sClient.List(ctx, secretList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, secret := range secretList.Items {
				err = k8sClient.Delete(ctx, &secret)
				Expect(err).NotTo(HaveOccurred())
			}

		})

		It("should reconcile a placement and update the targetref object", func() {
			// Create the test placement targeting the test gateway
			Expect(k8sClient.Create(ctx, placement)).To(BeNil())
			createdPlacement := &v1alpha1.Placement{}
			placementType := types.NamespacedName{Name: placement.Name, Namespace: placement.Namespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, placementType, createdPlacement)
				return err == nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// The Placement status should match 1 cluster
			var decisions []v1alpha1.ClusterDecision
			Eventually(func() bool {
				err := k8sClient.Get(ctx, placementType, createdPlacement)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				decisions = createdPlacement.Status.Decisions
				return len(decisions) == 1
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(decisions[0].ClusterName).To(BeEquivalentTo("cluster1"))
			Expect(createdPlacement.Status.NumberOfSelectedClusters).To(BeEquivalentTo(1))

			// The Gateway should have a sync annotation on it for the matched cluster
			var syncAnnotation string
			createdGateway := &gatewayv1beta1.Gateway{}
			gatewayType := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayType, createdGateway)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				syncAnnotation = createdGateway.Annotations[fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "cluster1")]
				return syncAnnotation != ""
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(syncAnnotation).To(BeEquivalentTo("true"))

			// Update the Placement to match the other cluster
			err := k8sClient.Get(ctx, placementType, createdPlacement)
			if err != nil {
				Fail("Error reading placement")
			}
			createdPlacement.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels = map[string]string{
				"region": "us-west-1",
			}
			err = k8sClient.Update(ctx, createdPlacement)
			if err != nil {
				Fail("Error updating placement")
			}

			// The Placement status should match the other cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, placementType, createdPlacement)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				decisions = createdPlacement.Status.Decisions
				return len(decisions) == 1 && decisions[0].ClusterName == "cluster2"
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(createdPlacement.Status.NumberOfSelectedClusters).To(BeEquivalentTo(1))

			// The Gateway should now have a sync annotation on it for the other cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayType, createdGateway)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				syncAnnotation = createdGateway.Annotations[fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "cluster2")]
				return syncAnnotation != ""
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(syncAnnotation).To(BeEquivalentTo("true"))

			// Update the Placement to match both clusters
			err = k8sClient.Get(ctx, placementType, createdPlacement)
			if err != nil {
				Fail("Error reading placement")
			}
			createdPlacement.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels = map[string]string{
				"country": "us",
			}
			err = k8sClient.Update(ctx, createdPlacement)
			if err != nil {
				Fail("Error updating placement")
			}

			// The Placement status should match both clusters
			Eventually(func() bool {
				err := k8sClient.Get(ctx, placementType, createdPlacement)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				decisions = createdPlacement.Status.Decisions
				return len(decisions) == 2
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			// TODO: Is ordering important here?
			Expect(decisions[0].ClusterName).To(BeEquivalentTo("cluster1"))
			Expect(decisions[1].ClusterName).To(BeEquivalentTo("cluster2"))
			Expect(createdPlacement.Status.NumberOfSelectedClusters).To(BeEquivalentTo(2))

			// The Gateway should now have 2 sync annotations
			var syncAnnotation1 string
			var syncAnnotation2 string
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayType, createdGateway)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				syncAnnotation1 = createdGateway.Annotations[fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "cluster1")]
				syncAnnotation2 = createdGateway.Annotations[fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "cluster2")]
				return syncAnnotation1 != "" && syncAnnotation2 != ""
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(syncAnnotation1).To(BeEquivalentTo("true"))
			Expect(syncAnnotation2).To(BeEquivalentTo("true"))

			// Update the Placement to match *no* clusters
			err = k8sClient.Get(ctx, placementType, createdPlacement)
			if err != nil {
				Fail("Error reading placement")
			}
			createdPlacement.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels = map[string]string{
				"region": "eu-west-1",
			}
			err = k8sClient.Update(ctx, createdPlacement)
			if err != nil {
				Fail("Error updating placement")
			}

			// The Placement status should show *no* cluster matches
			Eventually(func() bool {
				err := k8sClient.Get(ctx, placementType, createdPlacement)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				decisions = createdPlacement.Status.Decisions
				return len(decisions) == 0
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(createdPlacement.Status.NumberOfSelectedClusters).To(BeEquivalentTo(0))

			// The Gateway should now have *no* sync annotations
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayType, createdGateway)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				syncAnnotation1 = createdGateway.Annotations[fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "cluster1")]
				syncAnnotation2 = createdGateway.Annotations[fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "cluster2")]
				return syncAnnotation1 == "" && syncAnnotation2 == ""
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			// Update a cluster secret label to match the Placement selector
			createdSecret := &corev1.Secret{}
			secretType := types.NamespacedName{Name: cluster1.Name, Namespace: cluster1.Namespace}
			err = k8sClient.Get(ctx, secretType, createdSecret)
			if err != nil {
				Fail("Error reading cluster secret")
			}
			createdSecret.Labels = map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
				"region":                         "eu-west-1",
				"country":                        "ie",
			}
			err = k8sClient.Update(ctx, createdSecret)
			if err != nil {
				Fail("Error updating placement")
			}

			// The Placement status should show the cluster with updated label
			Eventually(func() bool {
				err := k8sClient.Get(ctx, placementType, createdPlacement)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				decisions = createdPlacement.Status.Decisions
				return len(decisions) == 1 && decisions[0].ClusterName == "cluster1"
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(createdPlacement.Status.NumberOfSelectedClusters).To(BeEquivalentTo(1))

			// The Gateway should have the sync annoation for the updated cluster
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayType, createdGateway)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				syncAnnotation = createdGateway.Annotations[fmt.Sprintf("%s%s", syncer.MCTC_SYNC_ANNOTATION_PREFIX, "cluster1")]
				return syncAnnotation != ""
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(syncAnnotation).To(BeEquivalentTo("true"))
		})
	})
})
