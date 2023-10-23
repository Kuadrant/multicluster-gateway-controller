package dns

import "strings"

// ZoneIDFilter holds a list of zone ids to filter by
type ZoneIDFilter struct {
	ZoneIDs []string
}

// NewZoneIDFilter returns a new ZoneIDFilter given a list of zone ids
func NewZoneIDFilter(zoneIDs []string) ZoneIDFilter {
	return ZoneIDFilter{zoneIDs}
}

// Match checks whether a zone matches one of the provided zone ids
func (f ZoneIDFilter) Match(zoneID string) bool {
	// An empty filter includes all zones.
	if len(f.ZoneIDs) == 0 {
		return true
	}

	for _, id := range f.ZoneIDs {
		if strings.HasSuffix(zoneID, id) {
			return true
		}
	}

	return false
}

// IsConfigured returns true if DomainFilter is configured, false otherwise
func (f ZoneIDFilter) IsConfigured() bool {
	if len(f.ZoneIDs) == 1 {
		return f.ZoneIDs[0] != ""
	}
	return len(f.ZoneIDs) > 0
}

// DomainFilter holds a list of domains to filter by
type DomainFilter struct {
	DomainNames []string
}

// NewDomainFilter returns a new DomainFilter given a list of domain names
func NewDomainFilter(domainNames []string) DomainFilter {
	return DomainFilter{domainNames}
}

// Match checks whether a zone matches one of the provided domains
func (f DomainFilter) Match(domainName string) bool {
	// An empty filter includes all zones.
	if len(f.DomainNames) == 0 {
		return true
	}

	for _, id := range f.DomainNames {
		if strings.HasSuffix(domainName, id) {
			return true
		}
	}

	return false
}

// IsConfigured returns true if DomainFilter is configured, false otherwise
func (f DomainFilter) IsConfigured() bool {
	if len(f.DomainNames) == 1 {
		return f.DomainNames[0] != ""
	}
	return len(f.DomainNames) > 0
}
