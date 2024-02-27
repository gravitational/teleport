package v1

import (
	"google.golang.org/protobuf/types/known/wrapperspb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types/crownjewel"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
)

func ToProto(crownJewel *crownjewel.CrownJewel) *crownjewelv1.CrownJewel {
	teleportMatchers := make([]*crownjewelv1.TeleportMatcher, 0, len(crownJewel.Spec.TeleportMatchers))
	for _, matcher := range crownJewel.Spec.TeleportMatchers {
		teleportMatchers = append(teleportMatchers, &crownjewelv1.TeleportMatcher{
			Name:   matcher.Name,
			Kinds:  matcher.Kinds,
			Labels: toProtoLabels(matcher.Labels),
		})
	}

	awsMatchers := make([]*crownjewelv1.AWSMatcher, 0, len(crownJewel.Spec.AWSMatchers))
	for _, matcher := range crownJewel.Spec.AWSMatchers {
		tags := make([]*crownjewelv1.AWSTag, 0, len(matcher.Tags))
		for _, tag := range matcher.Tags {
			var value *wrapperspb.StringValue
			if tag.Value != nil {
				value = wrapperspb.String(*tag.Value)
			}

			tags = append(tags, &crownjewelv1.AWSTag{
				Key:   tag.Key,
				Value: value,
			})
		}
		awsMatchers = append(awsMatchers, &crownjewelv1.AWSMatcher{
			Types:   matcher.Types,
			Regions: matcher.Regions,
			Tags:    tags,
		})
	}

	return &crownjewelv1.CrownJewel{
		Header: headerv1.ToResourceHeaderProto(crownJewel.ResourceHeader),
		Spec: &crownjewelv1.CrownJewelSpec{
			TeleportMatchers: teleportMatchers,
			AwsMatchers:      awsMatchers,
		},
	}
}

func toProtoLabels(labels map[string][]string) []*crownjewelv1.TeleportLabel {
	protoLabels := make([]*crownjewelv1.TeleportLabel, 0, len(labels))
	for key, values := range labels {
		protoLabels = append(protoLabels, &crownjewelv1.TeleportLabel{
			Key:    key,
			Values: toWrappedString(values),
		})
	}
	return protoLabels
}

func toWrappedString(values []string) []*wrapperspb.StringValue {
	wrappedValues := make([]*wrapperspb.StringValue, 0, len(values))
	for _, value := range values {
		wrappedValues = append(wrappedValues, wrapperspb.String(value))
	}
	return wrappedValues
}

func FromProto(crownJewel *crownjewelv1.CrownJewel) *crownjewel.CrownJewel {
	teleportMatchers := make([]crownjewel.TeleportMatcher, 0, len(crownJewel.Spec.TeleportMatchers))
	for _, matcher := range crownJewel.Spec.TeleportMatchers {
		teleportMatchers = append(teleportMatchers, crownjewel.TeleportMatcher{
			Name:   matcher.Name,
			Kinds:  matcher.Kinds,
			Labels: fromProtoLabels(matcher.Labels),
		})
	}

	awsMatchers := make([]crownjewel.AWSMatcher, 0, len(crownJewel.Spec.AwsMatchers))
	for _, matcher := range crownJewel.Spec.AwsMatchers {
		tags := make([]crownjewel.AWSTag, 0, len(matcher.Tags))
		for _, tag := range matcher.Tags {
			var value *string
			if tag.Value != nil {
				value = &tag.Value.Value
			}

			tags = append(tags, crownjewel.AWSTag{
				Key:   tag.Key,
				Value: value,
			})
		}
		awsMatchers = append(awsMatchers, crownjewel.AWSMatcher{
			Types:   matcher.Types,
			Regions: matcher.Regions,
			Tags:    tags,
		})
	}

	return &crownjewel.CrownJewel{
		ResourceHeader: headerv1.FromResourceHeaderProto(crownJewel.Header),
		Spec: crownjewel.Spec{
			TeleportMatchers: teleportMatchers,
			AWSMatchers:      awsMatchers,
		},
	}
}

func fromProtoLabels(labels []*crownjewelv1.TeleportLabel) map[string][]string {
	protoLabels := make(map[string][]string, len(labels))
	for _, label := range labels {
		protoLabels[label.Key] = fromWrappedString(label.Values)
	}
	return protoLabels
}

func fromWrappedString(values []*wrapperspb.StringValue) []string {
	wrappedValues := make([]string, 0, len(values))
	for _, value := range values {
		wrappedValues = append(wrappedValues, value.Value)
	}
	return wrappedValues
}
