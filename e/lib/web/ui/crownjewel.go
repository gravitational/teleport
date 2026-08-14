package ui

import (
	"google.golang.org/protobuf/types/known/wrapperspb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
)

// CrownJewel is a UI representation of a Crown Jewel.
type CrownJewel struct {
	// Name is the name of the Crown Jewel.
	Name string `json:"name"`
	// Description is the description of the Crown Jewel.
	Description string `json:"description"`
	// Spec is the specification of the Crown Jewel.
	Spec CrownJewelSpec `json:"spec"`
}

// CrownJewelSpec is the specification of a Crown Jewel.
type CrownJewelSpec struct {
	// TeleportMatchers is the list of Teleport matchers.
	TeleportMatchers []TeleportMatcher `json:"teleport_matchers"`
	// AwsMatchers is the list of AWS matchers.
	AwsMatchers []AWSMatcher `json:"aws_matchers"`
	// Query is Access Graph query to match resources.
	Query string `json:"query"`
}

// TeleportMatcher is a Teleport matcher.
type TeleportMatcher struct {
	// Kinds is the list of kinds the matcher applies to, e.g. "db", "ssh", etc.
	Kinds []string `json:"kinds"`
	// Names is the list of names the matcher applies to.
	Names []string `json:"names"`
	// Labels is the list of labels the matcher applies to.
	Labels map[string][]string `json:"labels"`
}

// AWSMatcher is an AWS matcher.
type AWSMatcher struct {
	// Types is the list of types the matcher applies to, e.g. "ec2", "rds", etc.
	Types []string `json:"types"`
	// Regions is the list of regions the matcher applies to.
	Regions []string `json:"regions"`
	// Tags is the list of tags the matcher applies to.
	Tags map[string][]string `json:"tags"`
}

// ToCrownJewel converts a Crown Jewel to a UI representation.
func ToCrownJewel(cj *crownjewelv1.CrownJewel) *CrownJewel {
	return &CrownJewel{
		Name:        cj.GetMetadata().GetName(),
		Description: cj.GetMetadata().GetDescription(),
		Spec: CrownJewelSpec{
			Query:            cj.GetSpec().GetQuery(),
			TeleportMatchers: ToTeleportMatchers(cj.GetSpec().GetTeleportMatchers()),
			AwsMatchers:      ToAWSMatchers(cj.GetSpec().GetAwsMatchers()),
		},
	}
}

func ToCrownJewels(crownJewels []*crownjewelv1.CrownJewel) []*CrownJewel {
	var result []*CrownJewel
	for _, cj := range crownJewels {
		result = append(result, ToCrownJewel(cj))
	}
	return result
}

// ToAWSMatchers converts a list of AWS matchers to a UI representation.
func ToAWSMatchers(matchers []*crownjewelv1.AWSMatcher) []AWSMatcher {
	var result []AWSMatcher
	for _, m := range matchers {
		result = append(result, AWSMatcher{
			Regions: m.GetRegions(),
			Types:   m.GetTypes(),
			Tags:    toAWSTag(m.GetTags()),
		})
	}
	return result
}

// ToTeleportMatchers converts a list of Teleport matchers to a UI representation.
func ToTeleportMatchers(matchers []*crownjewelv1.TeleportMatcher) []TeleportMatcher {
	var result []TeleportMatcher
	for _, m := range matchers {
		result = append(result, TeleportMatcher{
			Kinds:  m.GetKinds(),
			Names:  m.GetNames(),
			Labels: toLabels(m.GetLabels()),
		})
	}
	return result
}

func toAWSTag(tags []*crownjewelv1.AWSTag) map[string][]string {
	result := make(map[string][]string)
	for _, t := range tags {
		result[t.GetKey()] = pbToStr(t.GetValues())
	}
	return result
}

func pbToStr(values []*wrapperspb.StringValue) []string {
	var result []string
	for _, v := range values {
		result = append(result, v.Value)
	}
	return result
}

func toLabels(labels []*labelv1.Label) map[string][]string {
	result := make(map[string][]string)
	for _, l := range labels {
		result[l.GetName()] = l.GetValues()
	}
	return result
}
