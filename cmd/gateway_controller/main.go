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

package main

import (
	"context"
	"embed"
	"flag"
	"os"
	"sync"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes/scheme"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	kuadrantv1beta1 "github.com/kuadrant/kuadrant-operator/api/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/ocm/hub"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/placement"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/policysync"
	//+kubebuilder:scaffold:imports
)

var (
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	setupLog             = ctrl.Log.WithName("setup")

	//go:embed addon-manager/manifests
	FS embed.FS
)

const (
	addonName = "kuadrant-addon"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme.Scheme))

	utilruntime.Must(gatewayapiv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1beta2.AddToScheme(scheme.Scheme))
	utilruntime.Must(workv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))

	//+kubebuilder:scaffold:scheme
}

func GetDefaultValues(cluster *clusterv1.ManagedCluster,
	addon *addonapiv1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {

	defaultIstioOperator := "istiocontrolplane"
	defaultIstioOperatorNS := "istio-system"
	defaultIstioConfigMap := "istio"
	defaultCatalog := "operatorhubio-catalog"
	defaultCatalogNS := "olm"
	defaultChannel := "stable"

	manifestConfig := struct {
		IstioOperator          string
		IstioConfigMapName     string
		IstioOperatorNamespace string
		ClusterName            string
		CatalogSource          string
		CatalogSourceNS        string
		Channel                string
	}{
		ClusterName:            cluster.Name,
		IstioOperator:          defaultIstioOperator,
		IstioConfigMapName:     defaultIstioConfigMap,
		IstioOperatorNamespace: defaultIstioOperatorNS,
		CatalogSource:          defaultCatalog,
		CatalogSourceNS:        defaultCatalogNS,
		Channel:                defaultChannel,
	}

	return addonfactory.StructToValues(manifestConfig), nil
}

func startAddonManager(ctx context.Context) {
	setupLog.Info("starting add-on manager")
	addonScheme := runtime.NewScheme()
	utilruntime.Must(operatorsv1alpha1.AddToScheme(addonScheme))
	utilruntime.Must(operatorsv1.AddToScheme(addonScheme))
	utilruntime.Must(kuadrantv1beta1.AddToScheme(addonScheme))

	kubeConfig := ctrl.GetConfigOrDie()

	addonMgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		setupLog.Error(err, "unable to setup addon manager")
		os.Exit(1)
	}

	agentAddon, err := addonfactory.NewAgentAddonFactory(addonName, FS, "addon-manager/manifests").
		WithAgentHealthProber(hub.AddonHealthProber()).
		WithScheme(addonScheme).
		WithGetValuesFuncs(GetDefaultValues, addonfactory.GetValuesFromAddonAnnotation).
		BuildTemplateAgentAddon()
	if err != nil {
		setupLog.Error(err, "failed to build agent addon")
		os.Exit(1)
	}
	err = addonMgr.AddAgent(agentAddon)
	if err != nil {
		setupLog.Error(err, "failed to add addon agent")
		os.Exit(1)
	}

	if err := addonMgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running addon manager")
		os.Exit(1)
	}

}

func startGatewayController(ctx context.Context) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		WebhookServer:          webhook.NewServer(webhook.Options{Port: 9443}),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "fb80029c-controller.kuadrant.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	placer := placement.NewOCMPlacer(mgr.GetClient())
	if err = (&gateway.GatewayClassReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GatewayClass")
		os.Exit(1)
	}

	dynamicClient := dynamic.NewForConfigOrDie(mgr.GetConfig())
	dynamicInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		0,
		corev1.NamespaceAll,
		nil,
	)

	policyInformersManager := policysync.NewPolicyInformersManager(dynamicInformerFactory)
	if err := policyInformersManager.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to start policy informers manager")
		os.Exit(1)
	}

	if err = (&gateway.GatewayReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		Placement:              placer,
		PolicyInformersManager: policyInformersManager,
		DynamicClient:          dynamicClient,
		WatchedPolicies:        map[schema.GroupVersionResource]cache.ResourceEventHandlerRegistration{},
	}).SetupWithManager(mgr, ctx); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Gateway")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")

	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running controller manager")
		os.Exit(1)
	}

}

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	ctx := ctrl.SetupSignalHandler()

	wg := sync.WaitGroup{}
	wg.Add(2)
	go startAddonManager(ctx)
	go startGatewayController(ctx)
	wg.Wait()

	<-ctx.Done()
}
