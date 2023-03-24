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
	"flag"
	"os"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"

	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/admission"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/dnsrecord"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/managedzone"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/placement"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns/aws"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/tls"
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
	//+kubebuilder:scaffold:scheme
}

const (
	//(cbrookes) This will be removed in the future when we have many tenant ns and way to map to them
	defaultCtrlNS = "multi-cluster-traffic-controller-system"
)

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookPortNumber int
	var certProvider string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&webhookPortNumber, "webhooks-port", 8082, "The port of the webhooks server. Set to 0 disables the webhooks server")
	flag.StringVar(&certProvider, "cert-provider", "glbc-ca", "The name of the certificate provider to use")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

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

	awsAccessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	awsRegion := os.Getenv("AWS_REGION")
	dnsProviderConfig := v1alpha1.DNSProviderConfig{
		Route53: &v1alpha1.DNSProviderConfigRoute53{
			AccessKeyID:     awsAccessKeyID,
			SecretAccessKey: awsSecretAccessKey,
			Region:          awsRegion,
		},
	}
	dnsProvider, err := aws.NewDNSProvider(*dnsProviderConfig.Route53)
	if err != nil {
		setupLog.Error(err, "unable to create dns provider client")
		os.Exit(1)
	}
	if err = (&dnsrecord.DNSRecordReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		DNSProvider: dnsProvider,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNSRecord")
		os.Exit(1)
	}
	dnsService := dns.NewService(mgr.GetClient(), dns.NewSafeHostResolver(dns.NewDefaultHostResolver()), defaultCtrlNS)
	certService := tls.NewService(mgr.GetClient(), defaultCtrlNS, certProvider)

	if err = (&managedzone.ManagedZoneReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		DNSProvider: dnsProvider,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagedZone")
		os.Exit(1)
	}
	if err = (&gateway.GatewayClassReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GatewayClass")
		os.Exit(1)
	}

	if err = (&gateway.GatewayReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Certificates: certService,
		Host:         dnsService, // implements the required interface
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Gateway")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err = (&placement.PlacementReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Placement")
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

	if webhookPortNumber != 0 {
		setupLog.Info("starting webhook server")
		if err := mgr.Add(admission.NewWebhookServer(dnsService, certService, webhookPortNumber)); err != nil {
			setupLog.Error(err, "unable to set up webhook server")
			os.Exit(1)
		}
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1alpha1.ManagedZone{},
		"spec.domainName",
		func(obj client.Object) []string {
			return []string{obj.(*v1alpha1.ManagedZone).Spec.DomainName}
		},
	); err != nil {
		setupLog.Error(err, "unable to create index for managed zones")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
