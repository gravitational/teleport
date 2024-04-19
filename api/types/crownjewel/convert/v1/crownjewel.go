package v1

import (
	"google.golang.org/protobuf/types/known/wrapperspb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types/crownjewel"
	"github.com/gravitational/teleport/api/types/header"
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
			values := make([]*wrapperspb.StringValue, 0, len(tag.Values))
			for _, value := range tag.Values {
				// todo *string thing
				values = append(values, wrapperspb.String(*value))
			}

			tags = append(tags, &crownjewelv1.AWSTag{
				Key:    tag.Key,
				Values: values,
			})
		}
		awsMatchers = append(awsMatchers, &crownjewelv1.AWSMatcher{
			Types:   matcher.Types,
			Regions: matcher.Regions,
			Tags:    tags,
		})
	}

	return &crownjewelv1.CrownJewel{
		Kind:     crownJewel.Kind,
		SubKind:  crownJewel.SubKind,
		Version:  crownJewel.Version,
		Metadata: headerv1.ToMetadataProto(crownJewel.Metadata),
		Spec: &crownjewelv1.CrownJewelSpec{
			TeleportMatchers: teleportMatchers,
			AwsMatchers:      awsMatchers,
		},
	}
}

func toProtoLabels(labels map[string][]string) []*labelv1.Label {
	protoLabels := make([]*labelv1.Label, 0, len(labels))
	for key, values := range labels {
		protoLabels = append(protoLabels, &labelv1.Label{
			Name:   key,
			Values: values,
		})
	}
	return protoLabels
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
			var values []*string
			for _, value := range tag.Values {
				values = append(values, strPtr(value.String()))
			}

			tags = append(tags, crownjewel.AWSTag{
				Key:    tag.Key,
				Values: values,
			})
		}
		awsMatchers = append(awsMatchers, crownjewel.AWSMatcher{
			Types:   matcher.Types,
			Regions: matcher.Regions,
			Tags:    tags,
		})
	}

	return &crownjewel.CrownJewel{
		ResourceHeader: header.ResourceHeader{
			Kind:     crownJewel.Kind,
			SubKind:  crownJewel.SubKind,
			Version:  crownJewel.Version,
			Metadata: headerv1.FromMetadataProto(crownJewel.Metadata),
		},
		Spec: crownjewel.Spec{
			TeleportMatchers: teleportMatchers,
			AWSMatchers:      awsMatchers,
		},
	}
}

func fromProtoLabels(labels []*labelv1.Label) map[string][]string {
	protoLabels := make(map[string][]string, len(labels))
	for _, label := range labels {
		protoLabels[label.Name] = label.Values
	}
	return protoLabels
}

func strPtr(s string) *string {
	return &s
}
