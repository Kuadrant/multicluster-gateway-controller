package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Params struct {
	// DownstreamClass specifies what GatewayClassName to set in the
	// downstream clusters. For example:
	DownstreamClass string `json:"downstreamClass,omitempty"`

	// PoliciesToSync specifies a listof Policy GVRs that will be watched
	// in the hub and synced to the spokes
	PoliciesToSync []ParamsGroupVersionResource `json:"experimentalPolicySync,omitempty"`
}

type ParamsGroupVersionResource struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

func (gvr ParamsGroupVersionResource) ToGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvr.Group,
		Version:  gvr.Version,
		Resource: gvr.Resource,
	}
}

func (p *Params) GetDownstreamClass() string {
	return p.DownstreamClass
}

var defaultParams Params = Params{
	DownstreamClass: "istio",
}

type InvalidParamsError struct {
	message string
}

var _ error = &InvalidParamsError{}

// Error implements error
func (e *InvalidParamsError) Error() string {
	return e.message
}

func IsInvalidParamsError(err error) (is bool) {
	_, is = err.(*InvalidParamsError)
	return
}

type ParamsResolver func(context.Context, client.Client, gatewayapiv1.ParametersReference) (*Params, error)

var paramsResolvers = map[schema.GroupKind]ParamsResolver{
	{Group: corev1.GroupName, Kind: "ConfigMap"}: fromNamespacedObject(fromConfigMap),
}

func fromNamespacedObject[T client.Object](getParams func(T) (*Params, error)) ParamsResolver {
	template := *new(T)
	objectType := reflect.TypeOf(template).Elem()

	return func(ctx context.Context, client client.Client, paramsRef gatewayapiv1.ParametersReference) (*Params, error) {
		if paramsRef.Namespace == nil || *paramsRef.Namespace == "" {
			return nil, &InvalidParamsError{"Namespace must be defined"}
		}

		obj := reflect.New(objectType).Interface().(T)
		namespace := string(*paramsRef.Namespace)

		if err := client.Get(ctx, types.NamespacedName{
			Name:      paramsRef.Name,
			Namespace: namespace,
		}, obj); err != nil {
			return nil, &InvalidParamsError{fmt.Sprintf("failed to get object %s/%s: %s", namespace, paramsRef.Name, err.Error())}
		}

		return getParams(obj)
	}
}

func fromConfigMap(configMap *corev1.ConfigMap) (*Params, error) {
	paramsRaw, ok := configMap.Data["params"]
	if !ok {
		return nil, &InvalidParamsError{"Parameters must be defined in \"params\" field of ConfigMap"}
	}

	result := &Params{}
	if err := json.Unmarshal([]byte(paramsRaw), result); err != nil {
		return nil, &InvalidParamsError{fmt.Sprintf("Failed to unmarshal params: %v", err)}
	}

	return result, nil
}

func getParams(ctx context.Context, c client.Client, gatewayClassName string) (*Params, error) {

	gatewayClass := &gatewayapiv1.GatewayClass{}
	err := c.Get(ctx, client.ObjectKey{Name: gatewayClassName}, gatewayClass)
	if err != nil {
		return nil, err
	}
	if gatewayClass.Spec.ParametersRef == nil {
		// Default parameters
		return &defaultParams, nil
	}

	groupKind := schema.GroupKind{
		Group: string(gatewayClass.Spec.ParametersRef.Group),
		Kind:  string(gatewayClass.Spec.ParametersRef.Kind),
	}

	resolveParams, ok := paramsResolvers[groupKind]
	if !ok {
		return nil, &InvalidParamsError{fmt.Sprintf("unable to retrieve parameters for GroupKind %s", groupKind.String())}
	}

	return resolveParams(ctx, c, *gatewayClass.Spec.ParametersRef)
}
