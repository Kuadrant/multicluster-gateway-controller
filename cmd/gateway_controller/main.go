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
	"flag"
	"os"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"

	corev1 "k8s.io/api/core/v1"
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
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/placement"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/policysync"
	//+kubebuilder:scaffold:imports
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme.Scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(certmanv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(gatewayapi.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1beta2.AddToScheme(scheme.Scheme))
	utilruntime.Must(workv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
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
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme.Scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
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
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
