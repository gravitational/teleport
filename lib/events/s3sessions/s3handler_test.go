//go:build dynamodb

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

package s3sessions

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/test"
)

// TestStreams tests various streaming upload scenarios
func TestStreams(t *testing.T) {
	handler, err := NewHandler(context.Background(), Config{
		Region: "us-west-1",
		Path:   "/test/",
		Bucket: "teleport-unit-tests",
	})
	require.NoError(t, err)

	defer handler.Close()

	// Stream with handler and many parts
	t.Run("StreamSinglePart", func(t *testing.T) {
		test.StreamSinglePart(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}

func TestACL(t *testing.T) {
	t.Parallel()
	baseUrl := "s3://mybucket/path"
	for _, tc := range []struct {
		desc, acl string
		isError   bool
	}{
		{"no ACL", "", false},
		{"correct ACL", "bucket-owner-full-control", false},
		{"incorrect ACL", "something-else", true},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			url, err := url.Parse(fmt.Sprintf("%s?acl=%s", baseUrl, tc.acl))
			require.NoError(t, err)
			conf := Config{}
			err = conf.SetFromURL(url, "")
			if tc.isError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.acl, conf.ACL)
			}
		})
	}
}

type mockS3Client struct {
	mock.Mock
	s3Client
}

func (m *mockS3Client) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*s3.HeadBucketOutput), args.Error(1)
}

func (m *mockS3Client) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*s3.CreateBucketOutput), args.Error(1)
}

func (m *mockS3Client) PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*s3.PutBucketVersioningOutput), args.Error(1)
}
func (m *mockS3Client) PutBucketEncryption(ctx context.Context, params *s3.PutBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.PutBucketEncryptionOutput, error) {
	args := m.Called(ctx, params, optFns)
	return args.Get(0).(*s3.PutBucketEncryptionOutput), args.Error(1)
}
func TestEnsureBucket(t *testing.T) {
	mockClient := &mockS3Client{}
	var gotRegion s3types.BucketLocationConstraint
	mockClient.On("HeadBucket", mock.Anything, mock.Anything, mock.Anything).
		Return((*s3.HeadBucketOutput)(nil), (error)(&s3types.NoSuchBucket{}))
	mockClient.On("CreateBucket", mock.Anything, mock.Anything, mock.Anything).
		Return((*s3.CreateBucketOutput)(nil), (error)(nil)).Run(func(args mock.Arguments) {
		createBucket := args.Get(1).(*s3.CreateBucketInput)
		gotRegion = createBucket.CreateBucketConfiguration.LocationConstraint
	})
	mockClient.On("PutBucketVersioning", mock.Anything, mock.Anything, mock.Anything).
		Return((*s3.PutBucketVersioningOutput)(nil), (error)(nil))
	mockClient.On("PutBucketEncryption", mock.Anything, mock.Anything, mock.Anything).
		Return((*s3.PutBucketEncryptionOutput)(nil), (error)(nil))

	handler := &Handler{
		Config: Config{
			Region: "us-east-2",
		},
		client: mockClient,
	}
	err := handler.ensureBucket(context.Background())
	require.NoError(t, err)
	require.Equal(t, s3types.BucketLocationConstraintUsEast2, gotRegion)

}
