package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmv1alpha2 "github.com/operator-framework/api/pkg/operators/v1alpha2"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	"open-cluster-management.io/addon-framework/pkg/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"k8s.io/apimachinery/pkg/types"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	 hub "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/ocm/hub"






)

//go:embed manifests
var FS embed.FS

const (
	addonName = "kuadrant-addon"
)



func main() {
	addonScheme := runtime.NewScheme()
	utilruntime.Must(olmv1alpha1.AddToScheme(addonScheme))
	utilruntime.Must(olmv1alpha2.AddToScheme(addonScheme))
	utilruntime.Must(operatorsv1.AddToScheme(addonScheme))


	fmt.Println("add-on")
	kubeConfig := ctrl.GetConfigOrDie()

	addonClient, err := addonv1alpha1client.NewForConfig(kubeConfig)
	if err != nil {
		return 
	}
	kubeClient, err := kubernetes.NewForConfig(kubeConfig) 
	if err != nil {
		return 
	}

	addonMgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		klog.Errorf("unable to setup addon manager: %v", err)
		os.Exit(1)
	}
	fmt.Println("health")

	healthProber := utils.NewDeploymentProber(types.NamespacedName{Name: "kuadrant-operator-controller-manager", Namespace: "operators"})


	fmt.Println(healthProber,"add-on2")
	agentAddon, err := addonfactory.NewAgentAddonFactory(addonName, FS, "manifests").
	WithConfigGVRs(
		schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
		addonfactory.AddOnDeploymentConfigGVR,
	).
	WithGetValuesFuncs(
		hub.GetDefaultValues,
		addonfactory.GetAddOnDeploymentConfigValues(
			addonfactory.NewAddOnDeploymentConfigGetter(addonClient),
			addonfactory.ToAddOnNodePlacementValues,
		),
		hub.GetImageValues(kubeClient),
	).
			
	WithAgentHealthProber(healthProber).WithScheme(addonScheme).BuildTemplateAgentAddon()
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


