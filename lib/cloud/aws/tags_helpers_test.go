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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stretchr/testify/require"
)

func TestTagsToLabels(t *testing.T) {
	t.Parallel()

	inputTags := []*rds.Tag{
		{
			Key:   aws.String("Env"),
			Value: aws.String("dev"),
		},
		{
			Key:   aws.String("aws:cloudformation:stack-id"),
			Value: aws.String("some-id"),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String("test"),
		},
	}
	t.Log(inputTags)

	expectLabels := map[string]string{
		"Name":                        "test",
		"Env":                         "dev",
		"aws:cloudformation:stack-id": "some-id",
	}

	actuallabels := TagsToLabels(inputTags)
	require.Equal(t, expectLabels, actuallabels)
}

func TestLabelsToTags(t *testing.T) {
	t.Parallel()

	inputLabels := map[string]string{
		"labelB": "valueB",
		"labelA": "valueA",
	}

	expectTags := []*elasticache.Tag{
		{
			Key:   aws.String("labelA"),
			Value: aws.String("valueA"),
		},
		{
			Key:   aws.String("labelB"),
			Value: aws.String("valueB"),
		},
	}

	actualTags := LabelsToTags[elasticache.Tag](inputLabels)
	require.Equal(t, expectTags, actualTags)
}
