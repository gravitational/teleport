/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package aws

import (
	"testing"

	rdsTypesV2 "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/stretchr/testify/require"
)

func TestTagsToLabels(t *testing.T) {
	t.Parallel()

	t.Run("rds", func(t *testing.T) {
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

		expectLabels := map[string]string{
			"Name":                        "test",
			"Env":                         "dev",
			"aws:cloudformation:stack-id": "some-id",
		}

		actuallabels := TagsToLabels(inputTags)
		require.Equal(t, expectLabels, actuallabels)
	})

	t.Run("ec2", func(t *testing.T) {
		inputTags := []*ec2.Tag{
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

		expectLabels := map[string]string{
			"Name":                        "test",
			"Env":                         "dev",
			"aws:cloudformation:stack-id": "some-id",
		}

		actuallabels := TagsToLabels(inputTags)
		require.Equal(t, expectLabels, actuallabels)
	})

	t.Run("rdsV2", func(t *testing.T) {
		inputTags := []rdsTypesV2.Tag{
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

		expectLabels := map[string]string{
			"Name":                        "test",
			"Env":                         "dev",
			"aws:cloudformation:stack-id": "some-id",
		}

		actuallabels := TagsToLabels(inputTags)
		require.Equal(t, expectLabels, actuallabels)
	})

}

func TestLabelsToTags(t *testing.T) {
	t.Parallel()

	t.Run("elasticcache", func(t *testing.T) {
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
	})

	t.Run("rdsV2", func(t *testing.T) {
		inputLabels := map[string]string{
			"labelB": "valueB",
			"labelA": "valueA",
		}

		expectTags := []rdsTypesV2.Tag{
			{
				Key:   aws.String("labelA"),
				Value: aws.String("valueA"),
			},
			{
				Key:   aws.String("labelB"),
				Value: aws.String("valueB"),
			},
		}

		actualTags := LabelsToRDSV2Tags(inputLabels)
		require.EqualValues(t, expectTags, actualTags)
	})
}
