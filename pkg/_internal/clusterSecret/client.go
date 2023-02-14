package clusterSecret

import (
	"fmt"
	"net/url"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TLSClientConfig struct {
	Insecure bool   `json:"insecure"`
	CaData   []byte `json:"caData,omitempty"`
	CertData []byte `json:"certData,omitempty"`
	KeyData  []byte `json:"keyData,omitempty"`
}

type ProviderConfig struct {
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	APIVersion string   `json:"apiVersion,omitempty"`
}

type ClusterConfig struct {
	BearerToken        string          `json:"bearerToken,omitempty"`
	Username           string          `json:"username,omitempty"`
	Password           string          `json:"password,omitempty"`
	TlsClientConfig    TLSClientConfig `json:"tlsClientConfig,omitempty"`
	ExecProviderConfig ProviderConfig  `json:"execProviderConfig,omitempty"`
}

func ClientFromSecret(secret *v1.Secret) (client.Client, error) {
	restConfig, err := RestConfigFromSecret(secret)
	if err != nil {
		return nil, fmt.Errorf("error creating rest config from secret: %v", err.Error())
	}

	return client.New(restConfig, client.Options{})
}

func ClientsetFromSecret(secret *v1.Secret) (*kubernetes.Clientset, error) {
	restConfig, err := RestConfigFromSecret(secret)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func RestConfigFromSecret(secret *v1.Secret) (*rest.Config, error) {
	clusterClientConfig := &ClusterConfig{}
	err := json.Unmarshal(secret.Data["config"], clusterClientConfig)
	if err != nil {
		return nil, err
	}

	hostUrl, err := url.Parse(string(secret.Data["server"]))
	if err != nil {
		return nil, err
	}

	rc := &rest.Config{
		Host:        hostUrl.Host,
		Username:    clusterClientConfig.Username,
		Password:    clusterClientConfig.Password,
		BearerToken: clusterClientConfig.BearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			ServerName: strings.SplitN(hostUrl.Host, ":", 2)[0],
			CertData:   clusterClientConfig.TlsClientConfig.CertData,
			KeyData:    clusterClientConfig.TlsClientConfig.KeyData,
			CAData:     clusterClientConfig.TlsClientConfig.CaData,
		},
	}

	return rc, nil
}
