package traffic

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type CreateOrUpdateTraffic func(ctx context.Context, i Interface) error
type DeleteTraffic func(ctx context.Context, i Interface) error

type Interface interface {
	runtime.Object
	metav1.Object
	GetKind() string
	GetHosts() []string
	GetCacheKey() string
	GetNamespaceName() types.NamespacedName
	AddTLS(host string, secret *corev1.Secret)
	RemoveTLS(host []string)
	GetSpec() interface{}
}

type Pending struct {
	Rules []networkingv1.IngressRule `json:"rules"`
}
