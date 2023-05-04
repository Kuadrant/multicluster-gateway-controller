package main

import (
	"context"
	"embed"
	"os"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	ctrl "sigs.k8s.io/controller-runtime"
	hub "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/ocm/hub"
	 






)

//go:embed addon-manager/manifests
var FS embed.FS

const (
	addonName = "kuadrant-addon"
)



func main() {
	addonScheme := runtime.NewScheme()
	utilruntime.Must(operatorsv1alpha1.AddToScheme(addonScheme))
	utilruntime.Must(operatorsv1.AddToScheme(addonScheme))


	kubeConfig := ctrl.GetConfigOrDie()

	addonMgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		klog.Errorf("unable to setup addon manager: %v", err)
		os.Exit(1)
	}

	agentAddon, err := addonfactory.NewAgentAddonFactory(addonName, FS, "addon-manager/manifests").
		
	WithAgentHealthProber(hub.AddonHealthProber()).
	WithScheme(addonScheme).
	BuildTemplateAgentAddon()
	if err != nil {
		klog.Errorf("failed to build agent addon %v", err)
		os.Exit(1)
	}

	err = addonMgr.AddAgent(agentAddon)
	if err != nil {
		klog.Errorf("failed to add addon agent: %v", err)
		os.Exit(1)
	}

	ctx := context.Background()
	go addonMgr.Start(ctx)

	<-ctx.Done()
}


