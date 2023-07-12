package v1alpha1

import "strings"

// HealthProtocol represents the protocol to use when making a health check request
type HealthProtocol string

const (
	HttpProtocol  HealthProtocol = "HTTP"
	HttpsProtocol HealthProtocol = "HTTPS"
)

func NewHealthProtocol(p string) HealthProtocol {
	switch strings.ToUpper(p) {
	case "HTTPS":
		return HttpsProtocol
	case "HTTP":
		return HttpProtocol
	}
	return HttpProtocol
}

func (p HealthProtocol) ToScheme() string {
	switch p {
	case HttpProtocol:
		return "http"
	case HttpsProtocol:
		return "https"
	default:
		return "http"
	}
}

func (p HealthProtocol) IsHttp() bool {
	return p == HttpProtocol
}

func (p HealthProtocol) IsHttps() bool {
	return p == HttpsProtocol
}
