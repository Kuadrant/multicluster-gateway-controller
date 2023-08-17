package conditions

type ConditionType string
type ConditionReason string

const (
	ConditionTypeReady ConditionType = "Ready"

	DNSPolicyAffected         ConditionType   = "DNSPolicyAffected"
	DNSPolicyReasonAccepted   ConditionReason = "Accepted"
	DNSPolicyReasonInvalid    ConditionReason = "Invalid"
	DNSPolicyReasonConflicted ConditionReason = "Conflicted"
)
