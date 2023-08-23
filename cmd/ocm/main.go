package main

import (
	"context"
	"embed"
	"fmt"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	kuadrantv1beta1 "github.com/kuadrant/kuadrant-operator/api/v1beta1"

	hub "github.com/Kuadrant/multicluster-gateway-controller/pkg/ocm/hub"
)

//go:embed addon-manager/manifests
var FS embed.FS

const (
	addonName = "kuadrant-addon"
)

func main() {
	fmt.Println("starting add-on manager")
	addonScheme := runtime.NewScheme()
	utilruntime.Must(operatorsv1alpha1.AddToScheme(addonScheme))
	utilruntime.Must(operatorsv1.AddToScheme(addonScheme))
	utilruntime.Must(kuadrantv1beta1.AddToScheme(addonScheme))

	kubeConfig := ctrl.GetConfigOrDie()

	addonMgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		klog.Errorf("unable to setup addon manager: %v", err)
		panic(err)
	}

	agentAddon, err := addonfactory.NewAgentAddonFactory(addonName, FS, "addon-manager/manifests").
		WithAgentHealthProber(hub.AddonHealthProber()).
		WithScheme(addonScheme).
		BuildTemplateAgentAddon()
	if err != nil {
		klog.Errorf("failed to build agent addon %v", err)
		panic(err)
	}

	err = addonMgr.AddAgent(agentAddon)
	if err != nil {
		klog.Errorf("failed to add addon agent: %v", err)
		panic(err)
	}

	ctx := context.Background()
	go func() {
		if err := addonMgr.Start(ctx); err != nil {
			panic(err)
		}
	}()

	<-ctx.Done()
}
