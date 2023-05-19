package aws

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
)

// unimplementedRoute53 implements the route53iface.Route53API interface with dummy
// methods, to be used when mocking the API and only a subset of the methods are
// used
type unimplementedRoute53 struct{}

var _ route53iface.Route53API = &unimplementedRoute53{}

// ActivateKeySigningKey implements route53iface.Route53API
func (*unimplementedRoute53) ActivateKeySigningKey(*route53.ActivateKeySigningKeyInput) (*route53.ActivateKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// ActivateKeySigningKeyRequest implements route53iface.Route53API
func (*unimplementedRoute53) ActivateKeySigningKeyRequest(*route53.ActivateKeySigningKeyInput) (*request.Request, *route53.ActivateKeySigningKeyOutput) {
	panic("unimplemented")
}

// ActivateKeySigningKeyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ActivateKeySigningKeyWithContext(context.Context, *route53.ActivateKeySigningKeyInput, ...request.Option) (*route53.ActivateKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// AssociateVPCWithHostedZone implements route53iface.Route53API
func (*unimplementedRoute53) AssociateVPCWithHostedZone(*route53.AssociateVPCWithHostedZoneInput) (*route53.AssociateVPCWithHostedZoneOutput, error) {
	panic("unimplemented")
}

// AssociateVPCWithHostedZoneRequest implements route53iface.Route53API
func (*unimplementedRoute53) AssociateVPCWithHostedZoneRequest(*route53.AssociateVPCWithHostedZoneInput) (*request.Request, *route53.AssociateVPCWithHostedZoneOutput) {
	panic("unimplemented")
}

// AssociateVPCWithHostedZoneWithContext implements route53iface.Route53API
func (*unimplementedRoute53) AssociateVPCWithHostedZoneWithContext(context.Context, *route53.AssociateVPCWithHostedZoneInput, ...request.Option) (*route53.AssociateVPCWithHostedZoneOutput, error) {
	panic("unimplemented")
}

// ChangeCidrCollection implements route53iface.Route53API
func (*unimplementedRoute53) ChangeCidrCollection(*route53.ChangeCidrCollectionInput) (*route53.ChangeCidrCollectionOutput, error) {
	panic("unimplemented")
}

// ChangeCidrCollectionRequest implements route53iface.Route53API
func (*unimplementedRoute53) ChangeCidrCollectionRequest(*route53.ChangeCidrCollectionInput) (*request.Request, *route53.ChangeCidrCollectionOutput) {
	panic("unimplemented")
}

// ChangeCidrCollectionWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ChangeCidrCollectionWithContext(context.Context, *route53.ChangeCidrCollectionInput, ...request.Option) (*route53.ChangeCidrCollectionOutput, error) {
	panic("unimplemented")
}

// ChangeResourceRecordSets implements route53iface.Route53API
func (*unimplementedRoute53) ChangeResourceRecordSets(*route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error) {
	panic("unimplemented")
}

// ChangeResourceRecordSetsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ChangeResourceRecordSetsRequest(*route53.ChangeResourceRecordSetsInput) (*request.Request, *route53.ChangeResourceRecordSetsOutput) {
	panic("unimplemented")
}

// ChangeResourceRecordSetsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ChangeResourceRecordSetsWithContext(context.Context, *route53.ChangeResourceRecordSetsInput, ...request.Option) (*route53.ChangeResourceRecordSetsOutput, error) {
	panic("unimplemented")
}

// ChangeTagsForResource implements route53iface.Route53API
func (*unimplementedRoute53) ChangeTagsForResource(*route53.ChangeTagsForResourceInput) (*route53.ChangeTagsForResourceOutput, error) {
	panic("unimplemented")
}

// ChangeTagsForResourceRequest implements route53iface.Route53API
func (*unimplementedRoute53) ChangeTagsForResourceRequest(*route53.ChangeTagsForResourceInput) (*request.Request, *route53.ChangeTagsForResourceOutput) {
	panic("unimplemented")
}

// ChangeTagsForResourceWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ChangeTagsForResourceWithContext(context.Context, *route53.ChangeTagsForResourceInput, ...request.Option) (*route53.ChangeTagsForResourceOutput, error) {
	panic("unimplemented")
}

// CreateCidrCollection implements route53iface.Route53API
func (*unimplementedRoute53) CreateCidrCollection(*route53.CreateCidrCollectionInput) (*route53.CreateCidrCollectionOutput, error) {
	panic("unimplemented")
}

// CreateCidrCollectionRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateCidrCollectionRequest(*route53.CreateCidrCollectionInput) (*request.Request, *route53.CreateCidrCollectionOutput) {
	panic("unimplemented")
}

// CreateCidrCollectionWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateCidrCollectionWithContext(context.Context, *route53.CreateCidrCollectionInput, ...request.Option) (*route53.CreateCidrCollectionOutput, error) {
	panic("unimplemented")
}

// CreateHealthCheck implements route53iface.Route53API
func (*unimplementedRoute53) CreateHealthCheck(*route53.CreateHealthCheckInput) (*route53.CreateHealthCheckOutput, error) {
	panic("unimplemented")
}

// CreateHealthCheckRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateHealthCheckRequest(*route53.CreateHealthCheckInput) (*request.Request, *route53.CreateHealthCheckOutput) {
	panic("unimplemented")
}

// CreateHealthCheckWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateHealthCheckWithContext(context.Context, *route53.CreateHealthCheckInput, ...request.Option) (*route53.CreateHealthCheckOutput, error) {
	panic("unimplemented")
}

// CreateHostedZone implements route53iface.Route53API
func (*unimplementedRoute53) CreateHostedZone(*route53.CreateHostedZoneInput) (*route53.CreateHostedZoneOutput, error) {
	panic("unimplemented")
}

// CreateHostedZoneRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateHostedZoneRequest(*route53.CreateHostedZoneInput) (*request.Request, *route53.CreateHostedZoneOutput) {
	panic("unimplemented")
}

// CreateHostedZoneWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateHostedZoneWithContext(context.Context, *route53.CreateHostedZoneInput, ...request.Option) (*route53.CreateHostedZoneOutput, error) {
	panic("unimplemented")
}

// CreateKeySigningKey implements route53iface.Route53API
func (*unimplementedRoute53) CreateKeySigningKey(*route53.CreateKeySigningKeyInput) (*route53.CreateKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// CreateKeySigningKeyRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateKeySigningKeyRequest(*route53.CreateKeySigningKeyInput) (*request.Request, *route53.CreateKeySigningKeyOutput) {
	panic("unimplemented")
}

// CreateKeySigningKeyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateKeySigningKeyWithContext(context.Context, *route53.CreateKeySigningKeyInput, ...request.Option) (*route53.CreateKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// CreateQueryLoggingConfig implements route53iface.Route53API
func (*unimplementedRoute53) CreateQueryLoggingConfig(*route53.CreateQueryLoggingConfigInput) (*route53.CreateQueryLoggingConfigOutput, error) {
	panic("unimplemented")
}

// CreateQueryLoggingConfigRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateQueryLoggingConfigRequest(*route53.CreateQueryLoggingConfigInput) (*request.Request, *route53.CreateQueryLoggingConfigOutput) {
	panic("unimplemented")
}

// CreateQueryLoggingConfigWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateQueryLoggingConfigWithContext(context.Context, *route53.CreateQueryLoggingConfigInput, ...request.Option) (*route53.CreateQueryLoggingConfigOutput, error) {
	panic("unimplemented")
}

// CreateReusableDelegationSet implements route53iface.Route53API
func (*unimplementedRoute53) CreateReusableDelegationSet(*route53.CreateReusableDelegationSetInput) (*route53.CreateReusableDelegationSetOutput, error) {
	panic("unimplemented")
}

// CreateReusableDelegationSetRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateReusableDelegationSetRequest(*route53.CreateReusableDelegationSetInput) (*request.Request, *route53.CreateReusableDelegationSetOutput) {
	panic("unimplemented")
}

// CreateReusableDelegationSetWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateReusableDelegationSetWithContext(context.Context, *route53.CreateReusableDelegationSetInput, ...request.Option) (*route53.CreateReusableDelegationSetOutput, error) {
	panic("unimplemented")
}

// CreateTrafficPolicy implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicy(*route53.CreateTrafficPolicyInput) (*route53.CreateTrafficPolicyOutput, error) {
	panic("unimplemented")
}

// CreateTrafficPolicyInstance implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyInstance(*route53.CreateTrafficPolicyInstanceInput) (*route53.CreateTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// CreateTrafficPolicyInstanceRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyInstanceRequest(*route53.CreateTrafficPolicyInstanceInput) (*request.Request, *route53.CreateTrafficPolicyInstanceOutput) {
	panic("unimplemented")
}

// CreateTrafficPolicyInstanceWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyInstanceWithContext(context.Context, *route53.CreateTrafficPolicyInstanceInput, ...request.Option) (*route53.CreateTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// CreateTrafficPolicyRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyRequest(*route53.CreateTrafficPolicyInput) (*request.Request, *route53.CreateTrafficPolicyOutput) {
	panic("unimplemented")
}

// CreateTrafficPolicyVersion implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyVersion(*route53.CreateTrafficPolicyVersionInput) (*route53.CreateTrafficPolicyVersionOutput, error) {
	panic("unimplemented")
}

// CreateTrafficPolicyVersionRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyVersionRequest(*route53.CreateTrafficPolicyVersionInput) (*request.Request, *route53.CreateTrafficPolicyVersionOutput) {
	panic("unimplemented")
}

// CreateTrafficPolicyVersionWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyVersionWithContext(context.Context, *route53.CreateTrafficPolicyVersionInput, ...request.Option) (*route53.CreateTrafficPolicyVersionOutput, error) {
	panic("unimplemented")
}

// CreateTrafficPolicyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateTrafficPolicyWithContext(context.Context, *route53.CreateTrafficPolicyInput, ...request.Option) (*route53.CreateTrafficPolicyOutput, error) {
	panic("unimplemented")
}

// CreateVPCAssociationAuthorization implements route53iface.Route53API
func (*unimplementedRoute53) CreateVPCAssociationAuthorization(*route53.CreateVPCAssociationAuthorizationInput) (*route53.CreateVPCAssociationAuthorizationOutput, error) {
	panic("unimplemented")
}

// CreateVPCAssociationAuthorizationRequest implements route53iface.Route53API
func (*unimplementedRoute53) CreateVPCAssociationAuthorizationRequest(*route53.CreateVPCAssociationAuthorizationInput) (*request.Request, *route53.CreateVPCAssociationAuthorizationOutput) {
	panic("unimplemented")
}

// CreateVPCAssociationAuthorizationWithContext implements route53iface.Route53API
func (*unimplementedRoute53) CreateVPCAssociationAuthorizationWithContext(context.Context, *route53.CreateVPCAssociationAuthorizationInput, ...request.Option) (*route53.CreateVPCAssociationAuthorizationOutput, error) {
	panic("unimplemented")
}

// DeactivateKeySigningKey implements route53iface.Route53API
func (*unimplementedRoute53) DeactivateKeySigningKey(*route53.DeactivateKeySigningKeyInput) (*route53.DeactivateKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// DeactivateKeySigningKeyRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeactivateKeySigningKeyRequest(*route53.DeactivateKeySigningKeyInput) (*request.Request, *route53.DeactivateKeySigningKeyOutput) {
	panic("unimplemented")
}

// DeactivateKeySigningKeyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeactivateKeySigningKeyWithContext(context.Context, *route53.DeactivateKeySigningKeyInput, ...request.Option) (*route53.DeactivateKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// DeleteCidrCollection implements route53iface.Route53API
func (*unimplementedRoute53) DeleteCidrCollection(*route53.DeleteCidrCollectionInput) (*route53.DeleteCidrCollectionOutput, error) {
	panic("unimplemented")
}

// DeleteCidrCollectionRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteCidrCollectionRequest(*route53.DeleteCidrCollectionInput) (*request.Request, *route53.DeleteCidrCollectionOutput) {
	panic("unimplemented")
}

// DeleteCidrCollectionWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteCidrCollectionWithContext(context.Context, *route53.DeleteCidrCollectionInput, ...request.Option) (*route53.DeleteCidrCollectionOutput, error) {
	panic("unimplemented")
}

// DeleteHealthCheck implements route53iface.Route53API
func (*unimplementedRoute53) DeleteHealthCheck(*route53.DeleteHealthCheckInput) (*route53.DeleteHealthCheckOutput, error) {
	panic("unimplemented")
}

// DeleteHealthCheckRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteHealthCheckRequest(*route53.DeleteHealthCheckInput) (*request.Request, *route53.DeleteHealthCheckOutput) {
	panic("unimplemented")
}

// DeleteHealthCheckWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteHealthCheckWithContext(context.Context, *route53.DeleteHealthCheckInput, ...request.Option) (*route53.DeleteHealthCheckOutput, error) {
	panic("unimplemented")
}

// DeleteHostedZone implements route53iface.Route53API
func (*unimplementedRoute53) DeleteHostedZone(*route53.DeleteHostedZoneInput) (*route53.DeleteHostedZoneOutput, error) {
	panic("unimplemented")
}

// DeleteHostedZoneRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteHostedZoneRequest(*route53.DeleteHostedZoneInput) (*request.Request, *route53.DeleteHostedZoneOutput) {
	panic("unimplemented")
}

// DeleteHostedZoneWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteHostedZoneWithContext(context.Context, *route53.DeleteHostedZoneInput, ...request.Option) (*route53.DeleteHostedZoneOutput, error) {
	panic("unimplemented")
}

// DeleteKeySigningKey implements route53iface.Route53API
func (*unimplementedRoute53) DeleteKeySigningKey(*route53.DeleteKeySigningKeyInput) (*route53.DeleteKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// DeleteKeySigningKeyRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteKeySigningKeyRequest(*route53.DeleteKeySigningKeyInput) (*request.Request, *route53.DeleteKeySigningKeyOutput) {
	panic("unimplemented")
}

// DeleteKeySigningKeyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteKeySigningKeyWithContext(context.Context, *route53.DeleteKeySigningKeyInput, ...request.Option) (*route53.DeleteKeySigningKeyOutput, error) {
	panic("unimplemented")
}

// DeleteQueryLoggingConfig implements route53iface.Route53API
func (*unimplementedRoute53) DeleteQueryLoggingConfig(*route53.DeleteQueryLoggingConfigInput) (*route53.DeleteQueryLoggingConfigOutput, error) {
	panic("unimplemented")
}

// DeleteQueryLoggingConfigRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteQueryLoggingConfigRequest(*route53.DeleteQueryLoggingConfigInput) (*request.Request, *route53.DeleteQueryLoggingConfigOutput) {
	panic("unimplemented")
}

// DeleteQueryLoggingConfigWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteQueryLoggingConfigWithContext(context.Context, *route53.DeleteQueryLoggingConfigInput, ...request.Option) (*route53.DeleteQueryLoggingConfigOutput, error) {
	panic("unimplemented")
}

// DeleteReusableDelegationSet implements route53iface.Route53API
func (*unimplementedRoute53) DeleteReusableDelegationSet(*route53.DeleteReusableDelegationSetInput) (*route53.DeleteReusableDelegationSetOutput, error) {
	panic("unimplemented")
}

// DeleteReusableDelegationSetRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteReusableDelegationSetRequest(*route53.DeleteReusableDelegationSetInput) (*request.Request, *route53.DeleteReusableDelegationSetOutput) {
	panic("unimplemented")
}

// DeleteReusableDelegationSetWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteReusableDelegationSetWithContext(context.Context, *route53.DeleteReusableDelegationSetInput, ...request.Option) (*route53.DeleteReusableDelegationSetOutput, error) {
	panic("unimplemented")
}

// DeleteTrafficPolicy implements route53iface.Route53API
func (*unimplementedRoute53) DeleteTrafficPolicy(*route53.DeleteTrafficPolicyInput) (*route53.DeleteTrafficPolicyOutput, error) {
	panic("unimplemented")
}

// DeleteTrafficPolicyInstance implements route53iface.Route53API
func (*unimplementedRoute53) DeleteTrafficPolicyInstance(*route53.DeleteTrafficPolicyInstanceInput) (*route53.DeleteTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// DeleteTrafficPolicyInstanceRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteTrafficPolicyInstanceRequest(*route53.DeleteTrafficPolicyInstanceInput) (*request.Request, *route53.DeleteTrafficPolicyInstanceOutput) {
	panic("unimplemented")
}

// DeleteTrafficPolicyInstanceWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteTrafficPolicyInstanceWithContext(context.Context, *route53.DeleteTrafficPolicyInstanceInput, ...request.Option) (*route53.DeleteTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// DeleteTrafficPolicyRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteTrafficPolicyRequest(*route53.DeleteTrafficPolicyInput) (*request.Request, *route53.DeleteTrafficPolicyOutput) {
	panic("unimplemented")
}

// DeleteTrafficPolicyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteTrafficPolicyWithContext(context.Context, *route53.DeleteTrafficPolicyInput, ...request.Option) (*route53.DeleteTrafficPolicyOutput, error) {
	panic("unimplemented")
}

// DeleteVPCAssociationAuthorization implements route53iface.Route53API
func (*unimplementedRoute53) DeleteVPCAssociationAuthorization(*route53.DeleteVPCAssociationAuthorizationInput) (*route53.DeleteVPCAssociationAuthorizationOutput, error) {
	panic("unimplemented")
}

// DeleteVPCAssociationAuthorizationRequest implements route53iface.Route53API
func (*unimplementedRoute53) DeleteVPCAssociationAuthorizationRequest(*route53.DeleteVPCAssociationAuthorizationInput) (*request.Request, *route53.DeleteVPCAssociationAuthorizationOutput) {
	panic("unimplemented")
}

// DeleteVPCAssociationAuthorizationWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DeleteVPCAssociationAuthorizationWithContext(context.Context, *route53.DeleteVPCAssociationAuthorizationInput, ...request.Option) (*route53.DeleteVPCAssociationAuthorizationOutput, error) {
	panic("unimplemented")
}

// DisableHostedZoneDNSSEC implements route53iface.Route53API
func (*unimplementedRoute53) DisableHostedZoneDNSSEC(*route53.DisableHostedZoneDNSSECInput) (*route53.DisableHostedZoneDNSSECOutput, error) {
	panic("unimplemented")
}

// DisableHostedZoneDNSSECRequest implements route53iface.Route53API
func (*unimplementedRoute53) DisableHostedZoneDNSSECRequest(*route53.DisableHostedZoneDNSSECInput) (*request.Request, *route53.DisableHostedZoneDNSSECOutput) {
	panic("unimplemented")
}

// DisableHostedZoneDNSSECWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DisableHostedZoneDNSSECWithContext(context.Context, *route53.DisableHostedZoneDNSSECInput, ...request.Option) (*route53.DisableHostedZoneDNSSECOutput, error) {
	panic("unimplemented")
}

// DisassociateVPCFromHostedZone implements route53iface.Route53API
func (*unimplementedRoute53) DisassociateVPCFromHostedZone(*route53.DisassociateVPCFromHostedZoneInput) (*route53.DisassociateVPCFromHostedZoneOutput, error) {
	panic("unimplemented")
}

// DisassociateVPCFromHostedZoneRequest implements route53iface.Route53API
func (*unimplementedRoute53) DisassociateVPCFromHostedZoneRequest(*route53.DisassociateVPCFromHostedZoneInput) (*request.Request, *route53.DisassociateVPCFromHostedZoneOutput) {
	panic("unimplemented")
}

// DisassociateVPCFromHostedZoneWithContext implements route53iface.Route53API
func (*unimplementedRoute53) DisassociateVPCFromHostedZoneWithContext(context.Context, *route53.DisassociateVPCFromHostedZoneInput, ...request.Option) (*route53.DisassociateVPCFromHostedZoneOutput, error) {
	panic("unimplemented")
}

// EnableHostedZoneDNSSEC implements route53iface.Route53API
func (*unimplementedRoute53) EnableHostedZoneDNSSEC(*route53.EnableHostedZoneDNSSECInput) (*route53.EnableHostedZoneDNSSECOutput, error) {
	panic("unimplemented")
}

// EnableHostedZoneDNSSECRequest implements route53iface.Route53API
func (*unimplementedRoute53) EnableHostedZoneDNSSECRequest(*route53.EnableHostedZoneDNSSECInput) (*request.Request, *route53.EnableHostedZoneDNSSECOutput) {
	panic("unimplemented")
}

// EnableHostedZoneDNSSECWithContext implements route53iface.Route53API
func (*unimplementedRoute53) EnableHostedZoneDNSSECWithContext(context.Context, *route53.EnableHostedZoneDNSSECInput, ...request.Option) (*route53.EnableHostedZoneDNSSECOutput, error) {
	panic("unimplemented")
}

// GetAccountLimit implements route53iface.Route53API
func (*unimplementedRoute53) GetAccountLimit(*route53.GetAccountLimitInput) (*route53.GetAccountLimitOutput, error) {
	panic("unimplemented")
}

// GetAccountLimitRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetAccountLimitRequest(*route53.GetAccountLimitInput) (*request.Request, *route53.GetAccountLimitOutput) {
	panic("unimplemented")
}

// GetAccountLimitWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetAccountLimitWithContext(context.Context, *route53.GetAccountLimitInput, ...request.Option) (*route53.GetAccountLimitOutput, error) {
	panic("unimplemented")
}

// GetChange implements route53iface.Route53API
func (*unimplementedRoute53) GetChange(*route53.GetChangeInput) (*route53.GetChangeOutput, error) {
	panic("unimplemented")
}

// GetChangeRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetChangeRequest(*route53.GetChangeInput) (*request.Request, *route53.GetChangeOutput) {
	panic("unimplemented")
}

// GetChangeWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetChangeWithContext(context.Context, *route53.GetChangeInput, ...request.Option) (*route53.GetChangeOutput, error) {
	panic("unimplemented")
}

// GetCheckerIpRanges implements route53iface.Route53API
func (*unimplementedRoute53) GetCheckerIpRanges(*route53.GetCheckerIpRangesInput) (*route53.GetCheckerIpRangesOutput, error) {
	panic("unimplemented")
}

// GetCheckerIpRangesRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetCheckerIpRangesRequest(*route53.GetCheckerIpRangesInput) (*request.Request, *route53.GetCheckerIpRangesOutput) {
	panic("unimplemented")
}

// GetCheckerIpRangesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetCheckerIpRangesWithContext(context.Context, *route53.GetCheckerIpRangesInput, ...request.Option) (*route53.GetCheckerIpRangesOutput, error) {
	panic("unimplemented")
}

// GetDNSSEC implements route53iface.Route53API
func (*unimplementedRoute53) GetDNSSEC(*route53.GetDNSSECInput) (*route53.GetDNSSECOutput, error) {
	panic("unimplemented")
}

// GetDNSSECRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetDNSSECRequest(*route53.GetDNSSECInput) (*request.Request, *route53.GetDNSSECOutput) {
	panic("unimplemented")
}

// GetDNSSECWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetDNSSECWithContext(context.Context, *route53.GetDNSSECInput, ...request.Option) (*route53.GetDNSSECOutput, error) {
	panic("unimplemented")
}

// GetGeoLocation implements route53iface.Route53API
func (*unimplementedRoute53) GetGeoLocation(*route53.GetGeoLocationInput) (*route53.GetGeoLocationOutput, error) {
	panic("unimplemented")
}

// GetGeoLocationRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetGeoLocationRequest(*route53.GetGeoLocationInput) (*request.Request, *route53.GetGeoLocationOutput) {
	panic("unimplemented")
}

// GetGeoLocationWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetGeoLocationWithContext(context.Context, *route53.GetGeoLocationInput, ...request.Option) (*route53.GetGeoLocationOutput, error) {
	panic("unimplemented")
}

// GetHealthCheck implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheck(*route53.GetHealthCheckInput) (*route53.GetHealthCheckOutput, error) {
	panic("unimplemented")
}

// GetHealthCheckCount implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckCount(*route53.GetHealthCheckCountInput) (*route53.GetHealthCheckCountOutput, error) {
	panic("unimplemented")
}

// GetHealthCheckCountRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckCountRequest(*route53.GetHealthCheckCountInput) (*request.Request, *route53.GetHealthCheckCountOutput) {
	panic("unimplemented")
}

// GetHealthCheckCountWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckCountWithContext(context.Context, *route53.GetHealthCheckCountInput, ...request.Option) (*route53.GetHealthCheckCountOutput, error) {
	panic("unimplemented")
}

// GetHealthCheckLastFailureReason implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckLastFailureReason(*route53.GetHealthCheckLastFailureReasonInput) (*route53.GetHealthCheckLastFailureReasonOutput, error) {
	panic("unimplemented")
}

// GetHealthCheckLastFailureReasonRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckLastFailureReasonRequest(*route53.GetHealthCheckLastFailureReasonInput) (*request.Request, *route53.GetHealthCheckLastFailureReasonOutput) {
	panic("unimplemented")
}

// GetHealthCheckLastFailureReasonWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckLastFailureReasonWithContext(context.Context, *route53.GetHealthCheckLastFailureReasonInput, ...request.Option) (*route53.GetHealthCheckLastFailureReasonOutput, error) {
	panic("unimplemented")
}

// GetHealthCheckRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckRequest(*route53.GetHealthCheckInput) (*request.Request, *route53.GetHealthCheckOutput) {
	panic("unimplemented")
}

// GetHealthCheckStatus implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckStatus(*route53.GetHealthCheckStatusInput) (*route53.GetHealthCheckStatusOutput, error) {
	panic("unimplemented")
}

// GetHealthCheckStatusRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckStatusRequest(*route53.GetHealthCheckStatusInput) (*request.Request, *route53.GetHealthCheckStatusOutput) {
	panic("unimplemented")
}

// GetHealthCheckStatusWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckStatusWithContext(context.Context, *route53.GetHealthCheckStatusInput, ...request.Option) (*route53.GetHealthCheckStatusOutput, error) {
	panic("unimplemented")
}

// GetHealthCheckWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetHealthCheckWithContext(context.Context, *route53.GetHealthCheckInput, ...request.Option) (*route53.GetHealthCheckOutput, error) {
	panic("unimplemented")
}

// GetHostedZone implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZone(*route53.GetHostedZoneInput) (*route53.GetHostedZoneOutput, error) {
	panic("unimplemented")
}

// GetHostedZoneCount implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneCount(*route53.GetHostedZoneCountInput) (*route53.GetHostedZoneCountOutput, error) {
	panic("unimplemented")
}

// GetHostedZoneCountRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneCountRequest(*route53.GetHostedZoneCountInput) (*request.Request, *route53.GetHostedZoneCountOutput) {
	panic("unimplemented")
}

// GetHostedZoneCountWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneCountWithContext(context.Context, *route53.GetHostedZoneCountInput, ...request.Option) (*route53.GetHostedZoneCountOutput, error) {
	panic("unimplemented")
}

// GetHostedZoneLimit implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneLimit(*route53.GetHostedZoneLimitInput) (*route53.GetHostedZoneLimitOutput, error) {
	panic("unimplemented")
}

// GetHostedZoneLimitRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneLimitRequest(*route53.GetHostedZoneLimitInput) (*request.Request, *route53.GetHostedZoneLimitOutput) {
	panic("unimplemented")
}

// GetHostedZoneLimitWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneLimitWithContext(context.Context, *route53.GetHostedZoneLimitInput, ...request.Option) (*route53.GetHostedZoneLimitOutput, error) {
	panic("unimplemented")
}

// GetHostedZoneRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneRequest(*route53.GetHostedZoneInput) (*request.Request, *route53.GetHostedZoneOutput) {
	panic("unimplemented")
}

// GetHostedZoneWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetHostedZoneWithContext(context.Context, *route53.GetHostedZoneInput, ...request.Option) (*route53.GetHostedZoneOutput, error) {
	panic("unimplemented")
}

// GetQueryLoggingConfig implements route53iface.Route53API
func (*unimplementedRoute53) GetQueryLoggingConfig(*route53.GetQueryLoggingConfigInput) (*route53.GetQueryLoggingConfigOutput, error) {
	panic("unimplemented")
}

// GetQueryLoggingConfigRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetQueryLoggingConfigRequest(*route53.GetQueryLoggingConfigInput) (*request.Request, *route53.GetQueryLoggingConfigOutput) {
	panic("unimplemented")
}

// GetQueryLoggingConfigWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetQueryLoggingConfigWithContext(context.Context, *route53.GetQueryLoggingConfigInput, ...request.Option) (*route53.GetQueryLoggingConfigOutput, error) {
	panic("unimplemented")
}

// GetReusableDelegationSet implements route53iface.Route53API
func (*unimplementedRoute53) GetReusableDelegationSet(*route53.GetReusableDelegationSetInput) (*route53.GetReusableDelegationSetOutput, error) {
	panic("unimplemented")
}

// GetReusableDelegationSetLimit implements route53iface.Route53API
func (*unimplementedRoute53) GetReusableDelegationSetLimit(*route53.GetReusableDelegationSetLimitInput) (*route53.GetReusableDelegationSetLimitOutput, error) {
	panic("unimplemented")
}

// GetReusableDelegationSetLimitRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetReusableDelegationSetLimitRequest(*route53.GetReusableDelegationSetLimitInput) (*request.Request, *route53.GetReusableDelegationSetLimitOutput) {
	panic("unimplemented")
}

// GetReusableDelegationSetLimitWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetReusableDelegationSetLimitWithContext(context.Context, *route53.GetReusableDelegationSetLimitInput, ...request.Option) (*route53.GetReusableDelegationSetLimitOutput, error) {
	panic("unimplemented")
}

// GetReusableDelegationSetRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetReusableDelegationSetRequest(*route53.GetReusableDelegationSetInput) (*request.Request, *route53.GetReusableDelegationSetOutput) {
	panic("unimplemented")
}

// GetReusableDelegationSetWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetReusableDelegationSetWithContext(context.Context, *route53.GetReusableDelegationSetInput, ...request.Option) (*route53.GetReusableDelegationSetOutput, error) {
	panic("unimplemented")
}

// GetTrafficPolicy implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicy(*route53.GetTrafficPolicyInput) (*route53.GetTrafficPolicyOutput, error) {
	panic("unimplemented")
}

// GetTrafficPolicyInstance implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyInstance(*route53.GetTrafficPolicyInstanceInput) (*route53.GetTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// GetTrafficPolicyInstanceCount implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyInstanceCount(*route53.GetTrafficPolicyInstanceCountInput) (*route53.GetTrafficPolicyInstanceCountOutput, error) {
	panic("unimplemented")
}

// GetTrafficPolicyInstanceCountRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyInstanceCountRequest(*route53.GetTrafficPolicyInstanceCountInput) (*request.Request, *route53.GetTrafficPolicyInstanceCountOutput) {
	panic("unimplemented")
}

// GetTrafficPolicyInstanceCountWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyInstanceCountWithContext(context.Context, *route53.GetTrafficPolicyInstanceCountInput, ...request.Option) (*route53.GetTrafficPolicyInstanceCountOutput, error) {
	panic("unimplemented")
}

// GetTrafficPolicyInstanceRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyInstanceRequest(*route53.GetTrafficPolicyInstanceInput) (*request.Request, *route53.GetTrafficPolicyInstanceOutput) {
	panic("unimplemented")
}

// GetTrafficPolicyInstanceWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyInstanceWithContext(context.Context, *route53.GetTrafficPolicyInstanceInput, ...request.Option) (*route53.GetTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// GetTrafficPolicyRequest implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyRequest(*route53.GetTrafficPolicyInput) (*request.Request, *route53.GetTrafficPolicyOutput) {
	panic("unimplemented")
}

// GetTrafficPolicyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) GetTrafficPolicyWithContext(context.Context, *route53.GetTrafficPolicyInput, ...request.Option) (*route53.GetTrafficPolicyOutput, error) {
	panic("unimplemented")
}

// ListCidrBlocks implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrBlocks(*route53.ListCidrBlocksInput) (*route53.ListCidrBlocksOutput, error) {
	panic("unimplemented")
}

// ListCidrBlocksPages implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrBlocksPages(*route53.ListCidrBlocksInput, func(*route53.ListCidrBlocksOutput, bool) bool) error {
	panic("unimplemented")
}

// ListCidrBlocksPagesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrBlocksPagesWithContext(context.Context, *route53.ListCidrBlocksInput, func(*route53.ListCidrBlocksOutput, bool) bool, ...request.Option) error {
	panic("unimplemented")
}

// ListCidrBlocksRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrBlocksRequest(*route53.ListCidrBlocksInput) (*request.Request, *route53.ListCidrBlocksOutput) {
	panic("unimplemented")
}

// ListCidrBlocksWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrBlocksWithContext(context.Context, *route53.ListCidrBlocksInput, ...request.Option) (*route53.ListCidrBlocksOutput, error) {
	panic("unimplemented")
}

// ListCidrCollections implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrCollections(*route53.ListCidrCollectionsInput) (*route53.ListCidrCollectionsOutput, error) {
	panic("unimplemented")
}

// ListCidrCollectionsPages implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrCollectionsPages(*route53.ListCidrCollectionsInput, func(*route53.ListCidrCollectionsOutput, bool) bool) error {
	panic("unimplemented")
}

// ListCidrCollectionsPagesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrCollectionsPagesWithContext(context.Context, *route53.ListCidrCollectionsInput, func(*route53.ListCidrCollectionsOutput, bool) bool, ...request.Option) error {
	panic("unimplemented")
}

// ListCidrCollectionsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrCollectionsRequest(*route53.ListCidrCollectionsInput) (*request.Request, *route53.ListCidrCollectionsOutput) {
	panic("unimplemented")
}

// ListCidrCollectionsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrCollectionsWithContext(context.Context, *route53.ListCidrCollectionsInput, ...request.Option) (*route53.ListCidrCollectionsOutput, error) {
	panic("unimplemented")
}

// ListCidrLocations implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrLocations(*route53.ListCidrLocationsInput) (*route53.ListCidrLocationsOutput, error) {
	panic("unimplemented")
}

// ListCidrLocationsPages implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrLocationsPages(*route53.ListCidrLocationsInput, func(*route53.ListCidrLocationsOutput, bool) bool) error {
	panic("unimplemented")
}

// ListCidrLocationsPagesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrLocationsPagesWithContext(context.Context, *route53.ListCidrLocationsInput, func(*route53.ListCidrLocationsOutput, bool) bool, ...request.Option) error {
	panic("unimplemented")
}

// ListCidrLocationsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrLocationsRequest(*route53.ListCidrLocationsInput) (*request.Request, *route53.ListCidrLocationsOutput) {
	panic("unimplemented")
}

// ListCidrLocationsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListCidrLocationsWithContext(context.Context, *route53.ListCidrLocationsInput, ...request.Option) (*route53.ListCidrLocationsOutput, error) {
	panic("unimplemented")
}

// ListGeoLocations implements route53iface.Route53API
func (*unimplementedRoute53) ListGeoLocations(*route53.ListGeoLocationsInput) (*route53.ListGeoLocationsOutput, error) {
	panic("unimplemented")
}

// ListGeoLocationsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListGeoLocationsRequest(*route53.ListGeoLocationsInput) (*request.Request, *route53.ListGeoLocationsOutput) {
	panic("unimplemented")
}

// ListGeoLocationsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListGeoLocationsWithContext(context.Context, *route53.ListGeoLocationsInput, ...request.Option) (*route53.ListGeoLocationsOutput, error) {
	panic("unimplemented")
}

// ListHealthChecks implements route53iface.Route53API
func (*unimplementedRoute53) ListHealthChecks(*route53.ListHealthChecksInput) (*route53.ListHealthChecksOutput, error) {
	panic("unimplemented")
}

// ListHealthChecksPages implements route53iface.Route53API
func (*unimplementedRoute53) ListHealthChecksPages(*route53.ListHealthChecksInput, func(*route53.ListHealthChecksOutput, bool) bool) error {
	panic("unimplemented")
}

// ListHealthChecksPagesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListHealthChecksPagesWithContext(context.Context, *route53.ListHealthChecksInput, func(*route53.ListHealthChecksOutput, bool) bool, ...request.Option) error {
	panic("unimplemented")
}

// ListHealthChecksRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListHealthChecksRequest(*route53.ListHealthChecksInput) (*request.Request, *route53.ListHealthChecksOutput) {
	panic("unimplemented")
}

// ListHealthChecksWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListHealthChecksWithContext(context.Context, *route53.ListHealthChecksInput, ...request.Option) (*route53.ListHealthChecksOutput, error) {
	panic("unimplemented")
}

// ListHostedZones implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZones(*route53.ListHostedZonesInput) (*route53.ListHostedZonesOutput, error) {
	panic("unimplemented")
}

// ListHostedZonesByName implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesByName(*route53.ListHostedZonesByNameInput) (*route53.ListHostedZonesByNameOutput, error) {
	panic("unimplemented")
}

// ListHostedZonesByNameRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesByNameRequest(*route53.ListHostedZonesByNameInput) (*request.Request, *route53.ListHostedZonesByNameOutput) {
	panic("unimplemented")
}

// ListHostedZonesByNameWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesByNameWithContext(context.Context, *route53.ListHostedZonesByNameInput, ...request.Option) (*route53.ListHostedZonesByNameOutput, error) {
	panic("unimplemented")
}

// ListHostedZonesByVPC implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesByVPC(*route53.ListHostedZonesByVPCInput) (*route53.ListHostedZonesByVPCOutput, error) {
	panic("unimplemented")
}

// ListHostedZonesByVPCRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesByVPCRequest(*route53.ListHostedZonesByVPCInput) (*request.Request, *route53.ListHostedZonesByVPCOutput) {
	panic("unimplemented")
}

// ListHostedZonesByVPCWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesByVPCWithContext(context.Context, *route53.ListHostedZonesByVPCInput, ...request.Option) (*route53.ListHostedZonesByVPCOutput, error) {
	panic("unimplemented")
}

// ListHostedZonesPages implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesPages(*route53.ListHostedZonesInput, func(*route53.ListHostedZonesOutput, bool) bool) error {
	panic("unimplemented")
}

// ListHostedZonesPagesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesPagesWithContext(context.Context, *route53.ListHostedZonesInput, func(*route53.ListHostedZonesOutput, bool) bool, ...request.Option) error {
	panic("unimplemented")
}

// ListHostedZonesRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesRequest(*route53.ListHostedZonesInput) (*request.Request, *route53.ListHostedZonesOutput) {
	panic("unimplemented")
}

// ListHostedZonesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListHostedZonesWithContext(context.Context, *route53.ListHostedZonesInput, ...request.Option) (*route53.ListHostedZonesOutput, error) {
	panic("unimplemented")
}

// ListQueryLoggingConfigs implements route53iface.Route53API
func (*unimplementedRoute53) ListQueryLoggingConfigs(*route53.ListQueryLoggingConfigsInput) (*route53.ListQueryLoggingConfigsOutput, error) {
	panic("unimplemented")
}

// ListQueryLoggingConfigsPages implements route53iface.Route53API
func (*unimplementedRoute53) ListQueryLoggingConfigsPages(*route53.ListQueryLoggingConfigsInput, func(*route53.ListQueryLoggingConfigsOutput, bool) bool) error {
	panic("unimplemented")
}

// ListQueryLoggingConfigsPagesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListQueryLoggingConfigsPagesWithContext(context.Context, *route53.ListQueryLoggingConfigsInput, func(*route53.ListQueryLoggingConfigsOutput, bool) bool, ...request.Option) error {
	panic("unimplemented")
}

// ListQueryLoggingConfigsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListQueryLoggingConfigsRequest(*route53.ListQueryLoggingConfigsInput) (*request.Request, *route53.ListQueryLoggingConfigsOutput) {
	panic("unimplemented")
}

// ListQueryLoggingConfigsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListQueryLoggingConfigsWithContext(context.Context, *route53.ListQueryLoggingConfigsInput, ...request.Option) (*route53.ListQueryLoggingConfigsOutput, error) {
	panic("unimplemented")
}

// ListResourceRecordSets implements route53iface.Route53API
func (*unimplementedRoute53) ListResourceRecordSets(*route53.ListResourceRecordSetsInput) (*route53.ListResourceRecordSetsOutput, error) {
	panic("unimplemented")
}

// ListResourceRecordSetsPages implements route53iface.Route53API
func (*unimplementedRoute53) ListResourceRecordSetsPages(*route53.ListResourceRecordSetsInput, func(*route53.ListResourceRecordSetsOutput, bool) bool) error {
	panic("unimplemented")
}

// ListResourceRecordSetsPagesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListResourceRecordSetsPagesWithContext(context.Context, *route53.ListResourceRecordSetsInput, func(*route53.ListResourceRecordSetsOutput, bool) bool, ...request.Option) error {
	panic("unimplemented")
}

// ListResourceRecordSetsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListResourceRecordSetsRequest(*route53.ListResourceRecordSetsInput) (*request.Request, *route53.ListResourceRecordSetsOutput) {
	panic("unimplemented")
}

// ListResourceRecordSetsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListResourceRecordSetsWithContext(context.Context, *route53.ListResourceRecordSetsInput, ...request.Option) (*route53.ListResourceRecordSetsOutput, error) {
	panic("unimplemented")
}

// ListReusableDelegationSets implements route53iface.Route53API
func (*unimplementedRoute53) ListReusableDelegationSets(*route53.ListReusableDelegationSetsInput) (*route53.ListReusableDelegationSetsOutput, error) {
	panic("unimplemented")
}

// ListReusableDelegationSetsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListReusableDelegationSetsRequest(*route53.ListReusableDelegationSetsInput) (*request.Request, *route53.ListReusableDelegationSetsOutput) {
	panic("unimplemented")
}

// ListReusableDelegationSetsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListReusableDelegationSetsWithContext(context.Context, *route53.ListReusableDelegationSetsInput, ...request.Option) (*route53.ListReusableDelegationSetsOutput, error) {
	panic("unimplemented")
}

// ListTagsForResource implements route53iface.Route53API
func (*unimplementedRoute53) ListTagsForResource(*route53.ListTagsForResourceInput) (*route53.ListTagsForResourceOutput, error) {
	panic("unimplemented")
}

// ListTagsForResourceRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListTagsForResourceRequest(*route53.ListTagsForResourceInput) (*request.Request, *route53.ListTagsForResourceOutput) {
	panic("unimplemented")
}

// ListTagsForResourceWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListTagsForResourceWithContext(context.Context, *route53.ListTagsForResourceInput, ...request.Option) (*route53.ListTagsForResourceOutput, error) {
	panic("unimplemented")
}

// ListTagsForResources implements route53iface.Route53API
func (*unimplementedRoute53) ListTagsForResources(*route53.ListTagsForResourcesInput) (*route53.ListTagsForResourcesOutput, error) {
	panic("unimplemented")
}

// ListTagsForResourcesRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListTagsForResourcesRequest(*route53.ListTagsForResourcesInput) (*request.Request, *route53.ListTagsForResourcesOutput) {
	panic("unimplemented")
}

// ListTagsForResourcesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListTagsForResourcesWithContext(context.Context, *route53.ListTagsForResourcesInput, ...request.Option) (*route53.ListTagsForResourcesOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicies implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicies(*route53.ListTrafficPoliciesInput) (*route53.ListTrafficPoliciesOutput, error) {
	panic("unimplemented")
}

// ListTrafficPoliciesRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPoliciesRequest(*route53.ListTrafficPoliciesInput) (*request.Request, *route53.ListTrafficPoliciesOutput) {
	panic("unimplemented")
}

// ListTrafficPoliciesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPoliciesWithContext(context.Context, *route53.ListTrafficPoliciesInput, ...request.Option) (*route53.ListTrafficPoliciesOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyInstances implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstances(*route53.ListTrafficPolicyInstancesInput) (*route53.ListTrafficPolicyInstancesOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesByHostedZone implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesByHostedZone(*route53.ListTrafficPolicyInstancesByHostedZoneInput) (*route53.ListTrafficPolicyInstancesByHostedZoneOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesByHostedZoneRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesByHostedZoneRequest(*route53.ListTrafficPolicyInstancesByHostedZoneInput) (*request.Request, *route53.ListTrafficPolicyInstancesByHostedZoneOutput) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesByHostedZoneWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesByHostedZoneWithContext(context.Context, *route53.ListTrafficPolicyInstancesByHostedZoneInput, ...request.Option) (*route53.ListTrafficPolicyInstancesByHostedZoneOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesByPolicy implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesByPolicy(*route53.ListTrafficPolicyInstancesByPolicyInput) (*route53.ListTrafficPolicyInstancesByPolicyOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesByPolicyRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesByPolicyRequest(*route53.ListTrafficPolicyInstancesByPolicyInput) (*request.Request, *route53.ListTrafficPolicyInstancesByPolicyOutput) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesByPolicyWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesByPolicyWithContext(context.Context, *route53.ListTrafficPolicyInstancesByPolicyInput, ...request.Option) (*route53.ListTrafficPolicyInstancesByPolicyOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesRequest(*route53.ListTrafficPolicyInstancesInput) (*request.Request, *route53.ListTrafficPolicyInstancesOutput) {
	panic("unimplemented")
}

// ListTrafficPolicyInstancesWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyInstancesWithContext(context.Context, *route53.ListTrafficPolicyInstancesInput, ...request.Option) (*route53.ListTrafficPolicyInstancesOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyVersions implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyVersions(*route53.ListTrafficPolicyVersionsInput) (*route53.ListTrafficPolicyVersionsOutput, error) {
	panic("unimplemented")
}

// ListTrafficPolicyVersionsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyVersionsRequest(*route53.ListTrafficPolicyVersionsInput) (*request.Request, *route53.ListTrafficPolicyVersionsOutput) {
	panic("unimplemented")
}

// ListTrafficPolicyVersionsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListTrafficPolicyVersionsWithContext(context.Context, *route53.ListTrafficPolicyVersionsInput, ...request.Option) (*route53.ListTrafficPolicyVersionsOutput, error) {
	panic("unimplemented")
}

// ListVPCAssociationAuthorizations implements route53iface.Route53API
func (*unimplementedRoute53) ListVPCAssociationAuthorizations(*route53.ListVPCAssociationAuthorizationsInput) (*route53.ListVPCAssociationAuthorizationsOutput, error) {
	panic("unimplemented")
}

// ListVPCAssociationAuthorizationsRequest implements route53iface.Route53API
func (*unimplementedRoute53) ListVPCAssociationAuthorizationsRequest(*route53.ListVPCAssociationAuthorizationsInput) (*request.Request, *route53.ListVPCAssociationAuthorizationsOutput) {
	panic("unimplemented")
}

// ListVPCAssociationAuthorizationsWithContext implements route53iface.Route53API
func (*unimplementedRoute53) ListVPCAssociationAuthorizationsWithContext(context.Context, *route53.ListVPCAssociationAuthorizationsInput, ...request.Option) (*route53.ListVPCAssociationAuthorizationsOutput, error) {
	panic("unimplemented")
}

// TestDNSAnswer implements route53iface.Route53API
func (*unimplementedRoute53) TestDNSAnswer(*route53.TestDNSAnswerInput) (*route53.TestDNSAnswerOutput, error) {
	panic("unimplemented")
}

// TestDNSAnswerRequest implements route53iface.Route53API
func (*unimplementedRoute53) TestDNSAnswerRequest(*route53.TestDNSAnswerInput) (*request.Request, *route53.TestDNSAnswerOutput) {
	panic("unimplemented")
}

// TestDNSAnswerWithContext implements route53iface.Route53API
func (*unimplementedRoute53) TestDNSAnswerWithContext(context.Context, *route53.TestDNSAnswerInput, ...request.Option) (*route53.TestDNSAnswerOutput, error) {
	panic("unimplemented")
}

// UpdateHealthCheck implements route53iface.Route53API
func (*unimplementedRoute53) UpdateHealthCheck(*route53.UpdateHealthCheckInput) (*route53.UpdateHealthCheckOutput, error) {
	panic("unimplemented")
}

// UpdateHealthCheckRequest implements route53iface.Route53API
func (*unimplementedRoute53) UpdateHealthCheckRequest(*route53.UpdateHealthCheckInput) (*request.Request, *route53.UpdateHealthCheckOutput) {
	panic("unimplemented")
}

// UpdateHealthCheckWithContext implements route53iface.Route53API
func (*unimplementedRoute53) UpdateHealthCheckWithContext(context.Context, *route53.UpdateHealthCheckInput, ...request.Option) (*route53.UpdateHealthCheckOutput, error) {
	panic("unimplemented")
}

// UpdateHostedZoneComment implements route53iface.Route53API
func (*unimplementedRoute53) UpdateHostedZoneComment(*route53.UpdateHostedZoneCommentInput) (*route53.UpdateHostedZoneCommentOutput, error) {
	panic("unimplemented")
}

// UpdateHostedZoneCommentRequest implements route53iface.Route53API
func (*unimplementedRoute53) UpdateHostedZoneCommentRequest(*route53.UpdateHostedZoneCommentInput) (*request.Request, *route53.UpdateHostedZoneCommentOutput) {
	panic("unimplemented")
}

// UpdateHostedZoneCommentWithContext implements route53iface.Route53API
func (*unimplementedRoute53) UpdateHostedZoneCommentWithContext(context.Context, *route53.UpdateHostedZoneCommentInput, ...request.Option) (*route53.UpdateHostedZoneCommentOutput, error) {
	panic("unimplemented")
}

// UpdateTrafficPolicyComment implements route53iface.Route53API
func (*unimplementedRoute53) UpdateTrafficPolicyComment(*route53.UpdateTrafficPolicyCommentInput) (*route53.UpdateTrafficPolicyCommentOutput, error) {
	panic("unimplemented")
}

// UpdateTrafficPolicyCommentRequest implements route53iface.Route53API
func (*unimplementedRoute53) UpdateTrafficPolicyCommentRequest(*route53.UpdateTrafficPolicyCommentInput) (*request.Request, *route53.UpdateTrafficPolicyCommentOutput) {
	panic("unimplemented")
}

// UpdateTrafficPolicyCommentWithContext implements route53iface.Route53API
func (*unimplementedRoute53) UpdateTrafficPolicyCommentWithContext(context.Context, *route53.UpdateTrafficPolicyCommentInput, ...request.Option) (*route53.UpdateTrafficPolicyCommentOutput, error) {
	panic("unimplemented")
}

// UpdateTrafficPolicyInstance implements route53iface.Route53API
func (*unimplementedRoute53) UpdateTrafficPolicyInstance(*route53.UpdateTrafficPolicyInstanceInput) (*route53.UpdateTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// UpdateTrafficPolicyInstanceRequest implements route53iface.Route53API
func (*unimplementedRoute53) UpdateTrafficPolicyInstanceRequest(*route53.UpdateTrafficPolicyInstanceInput) (*request.Request, *route53.UpdateTrafficPolicyInstanceOutput) {
	panic("unimplemented")
}

// UpdateTrafficPolicyInstanceWithContext implements route53iface.Route53API
func (*unimplementedRoute53) UpdateTrafficPolicyInstanceWithContext(context.Context, *route53.UpdateTrafficPolicyInstanceInput, ...request.Option) (*route53.UpdateTrafficPolicyInstanceOutput, error) {
	panic("unimplemented")
}

// WaitUntilResourceRecordSetsChanged implements route53iface.Route53API
func (*unimplementedRoute53) WaitUntilResourceRecordSetsChanged(*route53.GetChangeInput) error {
	panic("unimplemented")
}

// WaitUntilResourceRecordSetsChangedWithContext implements route53iface.Route53API
func (*unimplementedRoute53) WaitUntilResourceRecordSetsChangedWithContext(context.Context, *route53.GetChangeInput, ...request.WaiterOption) error {
	panic("unimplemented")
}
