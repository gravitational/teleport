package crownjewel

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
	"github.com/gravitational/teleport/api/utils"
)

var _ types.Resource = &CrownJewel{}

// CrownJewel is a resource that represents the crown jewel resource.
type CrownJewel struct {
	// ResourceHeader is the common resource header for all resources.
	header.ResourceHeader

	// Spec is the specification for the crown jewel.
	Spec Spec `json:"spec" yaml:"spec"`
}

func (c *CrownJewel) MatchSearch(searchValues []string) bool {
	fieldVals := append(utils.MapToStrings(c.GetAllLabels()), c.GetName())
	return types.MatchSearch(fieldVals, searchValues, nil)
}

// Spec is the specification for the crown jewel.
type Spec struct {
	// TeleportMatchers is a list of teleport matchers.
	TeleportMatchers []TeleportMatcher `json:"teleport_matchers" yaml:"teleport_matchers"`
	// AWSMatchers is a list of AWS matchers.
	AWSMatchers []AWSMatcher `json:"aws_matchers" yaml:"aws_matchers"`
}

// TeleportMatcher represents a matcher for Teleport resources.
type TeleportMatcher struct {
	// Name is the name of the resource.
	Name string `json:"name" yaml:"name"`
	// Kind is the kind of the resource: ssh, k8s, db, etc
	Kinds []string `json:"kinds" yaml:"kinds"`
	// Labels is a set of labels.
	Labels map[string][]string `json:"labels" yaml:"labels"`
}

// AWSMatcher represents a matcher for AWS resources.
type AWSMatcher struct {
	ARN string `json:"arn" yaml:"arn"`
	// Types are AWS database types to match, "ec2", "rds", "eks", etc
	Types []string `json:"types" yaml:"types"`
	// Regions are AWS regions to query for databases.
	Regions []string `json:"regions" yaml:"regions"`
	// Tags are AWS resource Tags to match.
	// Labels is a set of labels.
	Tags []AWSTag `json:"labels" yaml:"labels"`
}

// AWSTag represents an AWS tag.
type AWSTag struct {
	Key    string    `json:"key" yaml:"key"`
	Values []*string `json:"value" yaml:"value"`
}

// GetMetadata returns the resource metadata.
func (c *CrownJewel) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(c.Metadata)
}
