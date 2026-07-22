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

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/require"
)

func TestTagsToLabels(t *testing.T) {
	t.Parallel()

	t.Run("rds", func(t *testing.T) {
		inputTags := []rdstypes.Tag{
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
		inputTags := []ec2types.Tag{
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
