package dns

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/martinlindhe/base36"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

const (
	DefaultWeight                        = int(v1alpha1.DefaultWeight)
	DefaultGeo                   GeoCode = "default"
	LabelLBAttributeGeoCode              = "kuadrant.io/lb-attribute-geo-code"
	LabelLBAttributeCustomWeight         = "kuadrant.io/lb-attribute-custom-weight"
)

// MultiClusterGatewayTarget represents a Gateway that is placed on multiple clusters (ClusterGateway).
type MultiClusterGatewayTarget struct {
	Gateway               *gatewayv1beta1.Gateway
	ClusterGatewayTargets []ClusterGatewayTarget
	LoadBalancing         *v1alpha1.LoadBalancingSpec
}

func NewMultiClusterGatewayTarget(gateway *gatewayv1beta1.Gateway, clusterGateways []ClusterGateway, loadBalancing *v1alpha1.LoadBalancingSpec) *MultiClusterGatewayTarget {
	mcg := &MultiClusterGatewayTarget{Gateway: gateway, LoadBalancing: loadBalancing}
	mcg.setClusterGatewayTargets(clusterGateways)
	return mcg
}

func (t *MultiClusterGatewayTarget) GetName() string {
	return fmt.Sprintf("%s-%s", t.Gateway.Name, t.Gateway.Namespace)
}

func (t *MultiClusterGatewayTarget) GetShortCode() string {
	return ToBase36hash(t.GetName())
}

// GroupTargetsByGeo groups targets based on Geo Code.
func (t *MultiClusterGatewayTarget) GroupTargetsByGeo() map[GeoCode][]ClusterGatewayTarget {
	geoTargets := make(map[GeoCode][]ClusterGatewayTarget)
	for _, target := range t.ClusterGatewayTargets {
		geoTargets[target.GetGeo()] = append(geoTargets[target.GetGeo()], target)
	}
	return geoTargets
}

func (t *MultiClusterGatewayTarget) GetDefaultGeo() GeoCode {
	if t.LoadBalancing != nil && t.LoadBalancing.Geo != nil {
		geoCode := GeoCode(t.LoadBalancing.Geo.DefaultGeo)
		if geoCode.IsValid() {
			return geoCode
		}
	}
	return DefaultGeo
}

func (t *MultiClusterGatewayTarget) GetDefaultWeight() int {
	if t.LoadBalancing != nil && t.LoadBalancing.Weighted != nil {
		return int(t.LoadBalancing.Weighted.DefaultWeight)
	}
	return DefaultWeight
}

func (t *MultiClusterGatewayTarget) setClusterGatewayTargets(clusterGateways []ClusterGateway) {
	var cgTargets []ClusterGatewayTarget
	for _, cg := range clusterGateways {
		var customWeights []*v1alpha1.CustomWeight
		if t.LoadBalancing != nil && t.LoadBalancing.Weighted != nil {
			customWeights = t.LoadBalancing.Weighted.Custom
		}
		cgt := NewClusterGatewayTarget(cg, t.GetDefaultGeo(), t.GetDefaultWeight(), customWeights)
		cgTargets = append(cgTargets, cgt)
	}
	t.ClusterGatewayTargets = cgTargets
}

// ClusterGateway contains the addresses of a Gateway on a single cluster and the attributes of the target cluster.
type ClusterGateway struct {
	ClusterName       string
	GatewayAddresses  []gatewayv1beta1.GatewayAddress
	ClusterAttributes ClusterAttributes
}

type ClusterAttributes struct {
	CustomWeight *string
	Geo          *GeoCode
}

type GeoCode string

// IsContinentCode returns true if it's a valid continent code
func (gc GeoCode) IsContinentCode() bool {
	return slice.ContainsString(getContinentCodes(), string(gc))
}

// IsCountryCode returns true if it's a valid ISO_3166 Alpha 2 country code (https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2)
func (gc GeoCode) IsCountryCode() bool {
	return slice.ContainsString(getCountryCodes(), string(gc))
}

func (gc GeoCode) IsDefaultCode() bool {
	return gc == DefaultGeo
}

func (gc GeoCode) IsValid() bool {
	return gc.IsContinentCode() || gc.IsCountryCode()
}

// C-AF: Africa; C-AN: Antarctica; C-AS: Asia; C-EU: Europe; C-OC: Oceania; C-NA: North America; C-SA: South America
func getContinentCodes() []string {
	return []string{"C-AF", "C-AN", "C-AS", "C-EU", "C-OC", "C-NA", "C-SA"}
}

func getCountryCodes() []string {
	return GetISO3166Alpha2Codes()
}

func NewClusterGateway(cluster metav1.Object, gatewayAddresses []gatewayv1beta1.GatewayAddress) *ClusterGateway {
	cgw := &ClusterGateway{
		ClusterName:      cluster.GetName(),
		GatewayAddresses: gatewayAddresses,
	}
	cgw.setClusterAttributesFromObject(cluster)
	return cgw
}

func (t *ClusterGateway) setClusterAttributesFromObject(mc metav1.Object) {
	t.ClusterAttributes = ClusterAttributes{}
	labels := mc.GetLabels()
	if labels == nil {
		return
	}
	if gc, ok := labels[LabelLBAttributeGeoCode]; ok {
		geoCode := GeoCode(gc)
		if geoCode.IsValid() {
			t.ClusterAttributes.Geo = &geoCode
		}
	}
	if cw, ok := labels[LabelLBAttributeCustomWeight]; ok {
		t.ClusterAttributes.CustomWeight = &cw
	}
}

// ClusterGatewayTarget represents a cluster Gateway with geo and weighting info calculated
type ClusterGatewayTarget struct {
	*ClusterGateway
	Geo    *GeoCode
	Weight *int
}

func NewClusterGatewayTarget(cg ClusterGateway, defaultGeoCode GeoCode, defaultWeight int, customWeights []*v1alpha1.CustomWeight) ClusterGatewayTarget {
	target := ClusterGatewayTarget{
		ClusterGateway: &cg,
	}
	target.setGeo(defaultGeoCode)
	target.setWeight(defaultWeight, customWeights)
	return target
}

func (t *ClusterGatewayTarget) GetGeo() GeoCode {
	return *t.Geo
}

func (t *ClusterGatewayTarget) GetWeight() int {
	return *t.Weight
}

func (t *ClusterGatewayTarget) GetName() string {
	return t.ClusterName
}

func (t *ClusterGatewayTarget) GetShortCode() string {
	return ToBase36hash(t.GetName())
}

func (t *ClusterGatewayTarget) setGeo(defaultGeo GeoCode) {
	if defaultGeo == DefaultGeo || t.ClusterAttributes.Geo == nil {
		t.Geo = &defaultGeo
		return
	}
	t.Geo = t.ClusterAttributes.Geo
}

func (t *ClusterGatewayTarget) setWeight(defaultWeight int, customWeights []*v1alpha1.CustomWeight) {
	weight := &defaultWeight
	if t.ClusterAttributes.CustomWeight != nil && customWeights != nil {
		for k := range customWeights {
			cw := customWeights[k]
			if *t.ClusterAttributes.CustomWeight == cw.Value {
				customWeight := int(cw.Weight)
				weight = &customWeight
				break
			}
		}
	}
	t.Weight = weight
}

func ToBase36hash(s string) string {
	hash := sha256.Sum224([]byte(s))
	// convert the hash to base36 (alphanumeric) to decrease collision probabilities
	base36hash := strings.ToLower(base36.EncodeBytes(hash[:]))
	// use 6 chars of the base36hash, should be enough to avoid collisions and keep the code short enough
	return base36hash[:6]
}
