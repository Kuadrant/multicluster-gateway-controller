package main

import (
	"context"
	"embed"
	"fmt"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

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

func GetDefaultValues(cluster *clusterv1.ManagedCluster,
	addon *addonapiv1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {

	defaultIstioOperator := "istiocontrolplane"
	defaultIstioOperatorNS := "istio-system"
	defaultIstioConfigMap := "istio"
	defaultCatalog := "operatorhubio-catalog"
	defaultCatalogNS := "olm"

	manifestConfig := struct {
		IstioOperator          string
		IstioConfigMapName     string
		IstioOperatorNamespace string
		ClusterName            string
		CatalogSource          string
		CatalogSourceNS        string
	}{
		ClusterName:            cluster.Name,
		IstioOperator:          defaultIstioOperator,
		IstioConfigMapName:     defaultIstioConfigMap,
		IstioOperatorNamespace: defaultIstioOperatorNS,
		CatalogSource:          defaultCatalog,
		CatalogSourceNS:        defaultCatalogNS,
	}

	return addonfactory.StructToValues(manifestConfig), nil
}

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
		WithGetValuesFuncs(GetDefaultValues, addonfactory.GetValuesFromAddonAnnotation).
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
