/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	rdsTypesV2 "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
)

// ResourceTag is a generic interface that represents an AWS resource tag.
type ResourceTag interface {
	// TODO Go generic does not allow access common fields yet. List all types
	//  here and use a type switch for now.
	rdsTypesV2.Tag |
		*rds.Tag |
		*redshift.Tag |
		*elasticache.Tag |
		*memorydb.Tag |
		*redshiftserverless.Tag |
		*opensearchservice.Tag
}

// TagsToLabels converts a list of AWS resource tags to a label map.
func TagsToLabels[Tag ResourceTag](tags []Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}

	labels := make(map[string]string)
	for _, tag := range tags {
		key, value := resourceTagToKeyValue(tag)

		if types.IsValidLabelKey(key) {
			labels[key] = value
		} else {
			logrus.Debugf("Skipping AWS resource tag %q, not a valid label key.", key)
		}
	}
	return labels
}

func resourceTagToKeyValue[Tag ResourceTag](tag Tag) (string, string) {
	switch v := any(tag).(type) {
	case *rds.Tag:
		return aws.StringValue(v.Key), aws.StringValue(v.Value)
	case *redshift.Tag:
		return aws.StringValue(v.Key), aws.StringValue(v.Value)
	case *elasticache.Tag:
		return aws.StringValue(v.Key), aws.StringValue(v.Value)
	case *memorydb.Tag:
		return aws.StringValue(v.Key), aws.StringValue(v.Value)
	case *redshiftserverless.Tag:
		return aws.StringValue(v.Key), aws.StringValue(v.Value)
	case rdsTypesV2.Tag:
		return aws.StringValue(v.Key), aws.StringValue(v.Value)
	case *opensearchservice.Tag:
		return aws.StringValue(v.Key), aws.StringValue(v.Value)
	default:
		return "", ""
	}
}

// SettableTag is a generic interface that represents an AWS resource tag with
// SetKey and SetValue functions.
type SettableTag[T any] interface {
	SetKey(key string) *T
	SetValue(Value string) *T
	*T
}

// LabelsToTags converts a label map to a list of AWS resource tags.
func LabelsToTags[T any, PT SettableTag[T]](labels map[string]string) (tags []*T) {
	keys := maps.Keys(labels)
	slices.Sort(keys)

	for _, key := range keys {
		tag := PT(new(T))
		tag.SetKey(key)
		tag.SetValue(labels[key])

		tags = append(tags, (*T)(tag))
	}
	return
}

// LabelsToRDSV2Tags converts labels into [rdsTypesV2.Tag] list.
func LabelsToRDSV2Tags(labels map[string]string) []rdsTypesV2.Tag {
	keys := maps.Keys(labels)
	slices.Sort(keys)

	ret := make([]rdsTypesV2.Tag, 0, len(keys))
	for _, key := range keys {
		key := key
		value := labels[key]

		ret = append(ret, rdsTypesV2.Tag{
			Key:   &key,
			Value: &value,
		})
	}

	return ret
}
