package dns

const (
	TargetTypeHost = "HOST"
	TargetTypeIP   = "IP"
)

type Target struct {
	TargetType string
	Value      string
}
