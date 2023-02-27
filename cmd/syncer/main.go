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
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	//+kubebuilder:scaffold:imports

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/clusterSecret"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer/spec"
)

var (
	scheme            = runtime.NewScheme()
	setupLog          = ctrl.Log.WithName("setup")
	NEVER_SYNCED_GVRs = []string{"pods"}
)

const (
	DEFAULT_RESYNC = 0
	SYNCER_THREADS = 1
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var controlPlaneConfigSecretName string
	var controlPlaneConfigSecretNamespace string
	var syncedResources arrayFlags

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&controlPlaneConfigSecretName, "control-plane-config-name", "control-plane-cluster", "The name of the secret with the control plane client configuration")
	flag.StringVar(&controlPlaneConfigSecretNamespace, "control-plane-config-namespace", "mctc-system", "The namespace containing the secret with the control plane client configuration")
	flag.Var(&syncedResources, "synced-resources", "A list of GVRs to sync (e.g. ingresses.v1.networking.k8s.io)")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "fb80029c-sync.kuadrant.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
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

	syncerContext, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()
	syncRunnable, err := startSyncers(syncerContext, syncedResources, client.ObjectKey{Namespace: controlPlaneConfigSecretNamespace, Name: controlPlaneConfigSecretName}, mgr)
	if err != nil {
		setupLog.Error(err, "unable to start syncers")
		os.Exit(1)
	}

	if err := mgr.Add(syncRunnable); err != nil {
		setupLog.Error(err, "error starting syncers")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func startSyncers(ctx context.Context, GVRs []string, secretRef client.ObjectKey, mgr manager.Manager) (*syncer.SyncRunnable, error) {
	logger := log.FromContext(ctx)
	controlPlaneSecret := &corev1.Secret{}
	secretClient, err := client.New(mgr.GetConfig(), client.Options{})
	if err != nil {
		setupLog.Error(err, "unable to create sync secret client")
		os.Exit(1)
	}
	err = secretClient.Get(ctx, secretRef, controlPlaneSecret, &client.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve control plane config secret '%v', error: %v", secretRef.String(), err)

	}

	upstreamClusterConfig, err := clusterSecret.ClusterConfigFromSecret(controlPlaneSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to create cluster config from control plane secret: %v", err)
	}
	upstreamRestConfig, err := clusterSecret.RestConfigFromSecret(controlPlaneSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to create rest config from control plane secret: %v", err)
	}
	upstreamClient, err := clusterSecret.DynamicClientsetFromSecret(controlPlaneSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to create client from control plane rest config: %v", err)
	}
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(upstreamClient, DEFAULT_RESYNC)
	logger.Info("starting informer")
	informerFactory.Start(ctx.Done())
	logger.Info("waiting for cache sync")
	informerFactory.WaitForCacheSync(ctx.Done())
	logger.Info("cache sync complete")

	specSyncConfig := syncer.Config{
		UpstreamClientConfig: upstreamRestConfig,
		GVRs:                 GVRs,
		DownStreamClient:     mgr.GetClient(),
		InformerFactory:      informerFactory,
		ClusterID:            upstreamClusterConfig.Name,
		NeverSyncedGVRs:      NEVER_SYNCED_GVRs,
	}

	downstreamDynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("could not make dynamic downstream client from manager rest config: %v", err.Error())
	}
	SpecSyncer, err := spec.NewSpecSyncer(upstreamClusterConfig.Name, upstreamClusterConfig.Name, upstreamClient, downstreamDynamicClient)
	if err != nil {
		return nil, fmt.Errorf("could not create new spec syncer: %v", err.Error())
	}
	logger.Info("starting syncer", "name", upstreamClusterConfig.Name)

	go SpecSyncer.Start(ctx)

	syncRunnable := syncer.GetSyncerRunnable(ctx, specSyncConfig, syncer.InformerForGVR, SpecSyncer)
	return syncRunnable, nil

}
