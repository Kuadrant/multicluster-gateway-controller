//go:build unit

package testutil

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

const (
	Domain                 = "thecat.com"
	ValidTestHostname      = "boop." + Domain
	FailFetchDNSsubdinain  = "failfetch"
	FailCreateDNSSubdomain = "failcreate"
	FailEnsureCertHost     = "failCreateCert" + "." + Domain
	FailGetCertSecretHost  = "failGetCert" + "." + Domain
	FailEndpointsHostname  = "failEndpoints" + "." + Domain
	FailPlacementHostname  = "failPlacement" + "." + Domain
	Cluster                = "test_cluster_one"
	Namespace              = "boop-namespace"
	DummyCRName            = "boop"
	Placement              = "placement"
)

func BuildValidTestRequest(name, ns string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: ns,
		},
	}
}

func BuildTestCondition(conditionType v1beta1.GatewayConditionType, generation int64, message string) v1.Condition {
	return v1.Condition{
		Type:               string(conditionType),
		Status:             v1.ConditionTrue,
		ObservedGeneration: generation,
		Message:            message,
		Reason:             string(v1beta1.GatewayReasonProgrammed),
	}
}

func ConditionsEqual(got, want []v1.Condition) bool {
	if len(got) != len(want) {
		return false
	}
	for _, wantCase := range want {
		found := false
		for _, gotCase := range got {
			if wantCase.Type == gotCase.Type &&
				wantCase.Status == gotCase.Status &&
				wantCase.ObservedGeneration == gotCase.ObservedGeneration &&
				wantCase.Reason == gotCase.Reason &&
				strings.Contains(gotCase.Message, wantCase.Message) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func GotExpectedError(expected string, got error) bool {
	if expected == "" {
		return true
	}
	return strings.Contains(got.Error(), expected)
}

func GetTime() *v1.Time {
	now := v1.Now()
	return &now
}

func AssertNoErrorReconciliation() func(res ctrl.Result, err error, t *testing.T) {
	return func(res ctrl.Result, err error, t *testing.T) {
		if err != nil || !res.IsZero() {
			t.Errorf("failed. Expected no error and empty response but got: err: %s, res: %v", err, res)
		}
	}
}

func GetValidTestClient(initLists ...client.ObjectList) client.WithWatch {
	return fake.NewClientBuilder().
		WithScheme(GetValidTestScheme()).
		WithLists(initLists...).
		Build()
}

func GetValidTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	return scheme
}

func GetBasicScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = v1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}
