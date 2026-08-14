package beamsv1

import (
	"regexp"

	"github.com/gravitational/trace"

	compute "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/lib/utils/set"
)

var beamRegionRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

// ValidateBeamRegionSyntax validates the supported syntax for a Beam region.
func ValidateBeamRegionSyntax(region string) error {
	if !beamRegionRE.MatchString(region) {
		return trace.BadParameter("region %q is not a valid Beam region", region)
	}
	return nil
}

// beamRegionValidator is the optional allowlist for regions that Beam requests
// may target. A nil or empty map still enforces region syntax, but accepts any
// syntactically valid region.
type beamRegionValidator struct {
	set.Set[string]
}

func newBeamRegionValidator(regions []string) (beamRegionValidator, error) {
	valid := set.NewWithCapacity[string](len(regions))
	for _, region := range regions {
		if err := ValidateBeamRegionSyntax(region); err != nil {
			return beamRegionValidator{}, trace.Wrap(err)
		}
		valid.Add(region)
	}
	return beamRegionValidator{valid}, nil
}

// validate validates region syntax and, when an allowlist is configured, validates
// that the region is allowed. An empty region is accepted for single
// compute-service configurations.
func (v beamRegionValidator) validate(region string) error {
	if region == "" {
		return nil
	}
	if err := ValidateBeamRegionSyntax(region); err != nil {
		return trace.Wrap(err)
	}
	if v.Len() == 0 {
		return nil
	}
	if !v.Contains(region) {
		return trace.BadParameter("region %q is not a valid Beam region", region)
	}
	return nil
}

// requestedBeamRegion returns the region that should be sent to the compute
// service. Explicit overrides take precedence over the proxy-discovered region;
// when neither is present, DefaultRegion is used.
func (s *BeamsService) requestedBeamRegion(reqProxyRegion, reqOverrideRegion string) (string, error) {
	region := reqProxyRegion
	if reqOverrideRegion != "" {
		region = reqOverrideRegion
	}
	if region == "" {
		region = s.defaultRegion
	}
	if err := s.regionValidator.validate(region); err != nil {
		return "", trace.Wrap(err)
	}
	return region, nil
}

// clientForRegion validates that region is safe to route before asking the
// configured provider for a compute service client. It intentionally does not
// apply the request-region allowlist because stored Beam regions are returned
// by the authoritative compute service and must remain routable for cleanup.
func (s *BeamsService) clientForRegion(region string) (compute.BeamsOrchestratorServiceClient, error) {
	if region == "" && s.computeServiceClient != nil {
		return s.computeServiceClient, nil
	}
	if region != "" {
		if err := ValidateBeamRegionSyntax(region); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	client, err := s.computeServiceProvider.ClientForRegion(region)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}
