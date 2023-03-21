package clusterSecret

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CLUSTER_SECRET_LABEL       = "argocd.argoproj.io/secret-type"
	CLUSTER_SECRET_LABEL_VALUE = "cluster"
)

func IsClusterSecret(object metav1.Object) bool {
	secretType, ok := object.GetLabels()[CLUSTER_SECRET_LABEL]
	if !ok {
		return false
	}

	return secretType == CLUSTER_SECRET_LABEL_VALUE
}
