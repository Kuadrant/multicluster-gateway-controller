package clusterSecret

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer"
)

type Service struct {
	Client client.Client
}

func NewService(controlClient client.Client) *Service {
	return &Service{Client: controlClient}
}

// GetAllClusterSecrets returns an array of all cluster secrets available.
func (s *Service) GetAllClusterSecrets(ctx context.Context) ([]ClusterSecret, error) {
	secretList := &corev1.SecretList{}
	listOptions := client.MatchingLabels{
		"argocd.argoproj.io/secret-type": "cluster",
	}
	err := s.Client.List(ctx, secretList, listOptions)
	if err := client.IgnoreNotFound(err); err != nil {
		log.Log.Error(err, "Unable to fetch secrets")
		return nil, err
	}
	return ToClusterSecrets(secretList.Items), nil
}

// GetClusterSecretsFromAnnotations returns an array of cluster secrets based on the given objects sync annotations.
// If the wildcard `all` annotation is present, all cluster secrets available are returned.
func (s *Service) GetClusterSecretsFromAnnotations(ctx context.Context, obj metav1.Object) ([]ClusterSecret, error) {
	clusters := []ClusterSecret{}

	allClusters, err := s.GetAllClusterSecrets(ctx)
	if err != nil {
		return clusters, err
	}

	if metadata.HasAnnotation(obj, syncer.MGC_SYNC_ANNOTATION_PREFIX+syncer.MGC_SYNC_ANNOTATION_WILDCARD) {
		return allClusters, nil
	}

	clustersMap := map[string]ClusterSecret{}
	for _, cluster := range allClusters {
		clustersMap[cluster.Config.Name] = cluster
	}
	for annKey := range obj.GetAnnotations() {
		if strings.HasPrefix(annKey, syncer.MGC_SYNC_ANNOTATION_PREFIX) {
			_, clusterName, found := strings.Cut(annKey, syncer.MGC_SYNC_ANNOTATION_PREFIX)
			if found {
				if cluster, exists := clustersMap[clusterName]; exists {
					clusters = append(clusters, cluster)
				}
			}
		}
	}
	return clusters, nil
}
