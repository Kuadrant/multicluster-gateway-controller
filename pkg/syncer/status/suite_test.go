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

package status

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	//+kubebuilder:scaffold:imports

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg            *rest.Config
	k8sClient      client.Client
	testEnv        *envtest.Environment
	ctx            context.Context
	cancel         context.CancelFunc
	clusterID      = "test-cluster"
	GVRs           = []string{"gateways.v1beta1.gateway.networking.k8s.io", "secrets.v1"}
	controlPlaneNS = "mgc-tenant"
	dataPlaneNS    = "mgc-downstream"
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
	zapLogger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	log.SetLogger(zapLogger)
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

	err = gatewayv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	ctrl.SetLogger(zapLogger)

	dynamicClient, err := dynamic.NewForConfig(k8sManager.GetConfig())
	Expect(err).ToNot(HaveOccurred())

	//start syncers
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	statusSyncConfig := syncer.Config{
		GVRs:               GVRs,
		InformerFactory:    informerFactory,
		ClusterID:          clusterID,
		NeverSyncedGVRs:    []string{},
		UpstreamNamespaces: []string{dataPlaneNS},
		DownstreamNS:       controlPlaneNS,
		Mutators:           []syncer.Mutator{},
	}

	StatusSyncer, err := NewStatusSyncer(clusterID, dynamicClient, dynamicClient, statusSyncConfig)
	Expect(err).NotTo(HaveOccurred())

	go StatusSyncer.Start(ctx)

	statusSyncRunnable := syncer.GetSyncerRunnable(statusSyncConfig, syncer.InformerForGVR, StatusSyncer)

	go func() {
		err := statusSyncRunnable.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNS},
	}
	err = k8sClient.Create(ctx, ns)
	Expect(err).To(BeNil())

	ns = &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: dataPlaneNS},
	}
	err = k8sClient.Create(ctx, ns)
	Expect(err).To(BeNil())

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

var _ = Describe("Status Syncer", func() {
	Context("testing the status syncer", func() {
		var gateway *gatewayv1beta1.Gateway
		hostname := gatewayv1beta1.Hostname("test.host")
		BeforeEach(func() {
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gateway",
					Namespace: controlPlaneNS,
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "test",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "listener-1",
							Hostname: &hostname,
							Port:     443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
						},
					},
				},
			}
		})

		//ensure gateway is removed after each test.
		AfterEach(func() {
			// Delete gateway from control plane
			Expect(func() bool {
				err := k8sClient.Delete(ctx, gateway)
				return err == nil || apierrors.IsNotFound(err)
			}()).To(BeTrue())

			// Confirm it's removed downstream
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: dataPlaneNS}, &gatewayv1beta1.Gateway{})
				return apierrors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should annotate the status to the upstream resource", func() {
			//we will be creating the data plane gateway (as spec syncer is not running)
			dataplaneGateway := gateway.DeepCopy()
			dataplaneGateway.Namespace = dataPlaneNS

			//create control plane gateway
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			controlPlaneGateway := &gatewayv1beta1.Gateway{}
			gatewayNamespacedName := types.NamespacedName{Name: gateway.Name, Namespace: controlPlaneNS}

			//create data plane gateway with status
			Expect(k8sClient.Create(ctx, dataplaneGateway)).To(BeNil())

			dataplaneGateway.Status.Listeners = []gatewayv1beta1.ListenerStatus{
				{
					Name: "test-status-listener",
					SupportedKinds: []gatewayv1beta1.RouteGroupKind{
						{
							Kind: "ingress",
						},
					},
					Conditions: []metav1.Condition{
						{
							Type:               "testing",
							Status:             "True",
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             "testing",
							Message:            "testing",
						},
					},
				},
			}
			addressType := gatewayv1beta1.HostnameAddressType
			dataplaneGateway.Status.Addresses = []gatewayv1beta1.GatewayAddress{
				{
					Type:  &addressType,
					Value: "test.host",
				},
			}
			dataplaneGateway.Status.Conditions = []metav1.Condition{
				{
					Type:               "testing",
					Status:             "True",
					ObservedGeneration: 1,
					LastTransitionTime: metav1.Now(),
					Reason:             "testing",
					Message:            "testing",
				},
			}

			ctrl.Log.Info("updating data plane gateway status")
			Expect(k8sClient.Status().Update(ctx, dataplaneGateway.DeepCopy())).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: dataplaneGateway.Name, Namespace: dataplaneGateway.Namespace}, dataplaneGateway)
				if err != nil {
					ctrl.Log.Error(err, "error getting data plane gateway")
					return false
				}
				err = k8sClient.Get(ctx, gatewayNamespacedName, controlPlaneGateway)
				if err != nil {
					ctrl.Log.Error(err, "error getting control plane gateway")
					return false
				}
				//ctrl.Log.Info("got control plane gateway", "annotations", controlPlaneGateway.Annotations)
				if !metadata.HasAnnotation(controlPlaneGateway, SyncerClusterStatusAnnotationPrefix+clusterID) {
					return false
				}
				annotation := metadata.GetAnnotation(controlPlaneGateway, SyncerClusterStatusAnnotationPrefix+clusterID)
				controlPlaneStatus := &gatewayv1beta1.GatewayStatus{}
				err = json.Unmarshal([]byte(annotation), controlPlaneStatus)
				if err != nil {
					return false
				}

				if controlPlaneStatus.Listeners == nil {
					return false
				}
				if controlPlaneStatus.Addresses == nil {
					return false
				}
				if controlPlaneStatus.Conditions == nil {
					return false
				}
				return true
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, dataplaneGateway)).To(BeNil())
		})

	})
})
