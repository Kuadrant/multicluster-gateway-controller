package ocm

import (
	"context"
	"embed"
	"os"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	kuadrantv1beta1 "github.com/kuadrant/kuadrant-operator/api/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/ocm/hub"
)

//go:embed manifests
var FS embed.FS

const (
	addonName = "kuadrant-addon"
)

type AddonRunnable struct{}

func (r AddonRunnable) Start(ctx context.Context) error {
	setupLog := ctrl.Log.WithName("addon manager setup")
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

	agentAddon, err := addonfactory.NewAgentAddonFactory(addonName, FS, "manifests").
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

	if err = addonMgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running addon manager")
		return err
	}

	<-ctx.Done()

	return nil
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
