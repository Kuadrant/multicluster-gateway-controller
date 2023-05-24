package clusterSecret

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	MGC_ATTRIBUTE_ANNOTATION_PREFIX = "kuadrant.io/attribute-"
)

type ClusterSecret struct {
	corev1.Secret
	Config ClusterConfig
}

func NewClusterSecret(secret corev1.Secret) (*ClusterSecret, error) {
	if !IsClusterSecret(&secret) {
		return nil, fmt.Errorf("is not a cluster secret %v", secret)
	}
	config, err := ClusterConfigFromSecret(&secret)
	if err != nil {
		return nil, err
	}
	return &ClusterSecret{secret, *config}, nil
}

// ToClusterSecrets converts an array of v1.Secrets in to an array of ClusterSecrets.
// Any that cannot be converted are ignored,
func ToClusterSecrets(secrets []corev1.Secret) []ClusterSecret {
	clusters := []ClusterSecret{}
	for _, secret := range secrets {
		cs, err := NewClusterSecret(secret)
		if err != nil {
			continue
		}
		clusters = append(clusters, *cs)
	}
	return clusters
}

// GetAttributes returns a map of all key/value attributes added to the cluster secret.
// Attribute key and values are added as annotations on the cluster secret with the following format:
// kuadrant.io/attribute-<attribute name>=<attribute value>
func (cs *ClusterSecret) GetAttributes() map[string]string {
	attrs := map[string]string{}
	for annKey, annValue := range cs.GetAnnotations() {
		if strings.HasPrefix(annKey, MGC_ATTRIBUTE_ANNOTATION_PREFIX) {
			_, key, found := strings.Cut(annKey, MGC_ATTRIBUTE_ANNOTATION_PREFIX)
			if found {
				attrs[key] = annValue
			}
		}
	}
	return attrs
}
