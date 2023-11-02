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

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnspolicy"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnsrecord"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/managedzone"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/tlspolicy"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/dnsprovider"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme.Scheme))
	utilruntime.Must(gatewayapi.AddToScheme(scheme.Scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(certmanv1.AddToScheme(scheme.Scheme))
	//this is need for now but will be removed soon
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var ocmHub bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.BoolVar(&ocmHub, "ocm-hub", true,
		"tells the controller if it is in an ocm hub"+
			"setting this to false will cause the controller to stop watching for managedcluster resources")
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
		LeaderElectionID:       "fb80029c-policy-controller.kuadrant.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	provider := dnsprovider.NewProvider(mgr.GetClient())

	if err = (&dnsrecord.DNSRecordReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		DNSProvider: provider.DNSProviderFactory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNSRecord")
		os.Exit(1)
	}

	dnsPolicyBaseReconciler := reconcilers.NewBaseReconciler(
		mgr.GetClient(), mgr.GetScheme(), mgr.GetAPIReader(),
		log.Log.WithName("dnspolicy"),
		mgr.GetEventRecorderFor("DNSPolicy"),
	)

	if err = (&dnspolicy.DNSPolicyReconciler{
		TargetRefReconciler: reconcilers.TargetRefReconciler{
			BaseReconciler: dnsPolicyBaseReconciler,
		},
		DNSProvider: provider.DNSProviderFactory,
	}).SetupWithManager(mgr, ocmHub); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNSPolicy")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	tlsPolicyBaseReconciler := reconcilers.NewBaseReconciler(
		mgr.GetClient(), mgr.GetScheme(), mgr.GetAPIReader(),
		log.Log.WithName("tlspolicy"),
		mgr.GetEventRecorderFor("TLSPolicy"),
	)

	if err = (&tlspolicy.TLSPolicyReconciler{
		TargetRefReconciler: reconcilers.TargetRefReconciler{
			BaseReconciler: tlsPolicyBaseReconciler,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "TLSPolicy")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err = (&managedzone.ManagedZoneReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		DNSProvider: provider.DNSProviderFactory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagedZone")
		os.Exit(1)
	}

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
