package shared

const (
	DNSPoliciesBackRefAnnotation = "kuadrant.io/dnspolicies"
)

type DNSPolicyRefsConfig struct{}

func (c *DNSPolicyRefsConfig) PolicyRefsAnnotation() string {
	return DNSPoliciesBackRefAnnotation
}
