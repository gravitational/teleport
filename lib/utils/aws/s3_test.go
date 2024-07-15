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
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

func TestNewS3V2FileWriter(t *testing.T) {
	t.Run("writer.Close waits for upload to finish", func(t *testing.T) {
		ctx := context.Background()
		mock := &s3ClientMock{}
		writer, err := NewS3V2FileWriter(ctx, mock, "bucket", "key", nil /* uploader options */)
		require.NoError(t, err)

		testMessage := "test message"
		_, err = fmt.Fprint(writer, testMessage)
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		require.True(t, mock.getPutOjectCalled())
		require.Equal(t, testMessage, string(mock.getPutOjectBody()))
	})

	t.Run("on failure from upload, error is propagated", func(t *testing.T) {
		ctx := context.Background()
		mock := &s3ClientMock{
			putObjectErr: errors.New("error from reading"),
		}
		writer, err := NewS3V2FileWriter(ctx, mock, "bucket", "key", nil /* uploader options */)
		require.NoError(t, err)

		testMessage := "test message"
		_, err = fmt.Fprint(writer, testMessage)
		require.NoError(t, err)

		// Error should be received on close.
		err = writer.Close()
		require.ErrorContains(t, err, "error from reading")
	})
}

type s3ClientMock struct {
	manager.UploadAPIClient

	putObjectErr error

	// mu protects putObject fields
	mu              sync.Mutex
	putObjectCalled bool
	putObjectBody   []byte
}

func (s *s3ClientMock) getPutOjectCalled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putObjectCalled
}

func (s *s3ClientMock) getPutOjectBody() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putObjectBody
}

func (s *s3ClientMock) PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if s.putObjectErr != nil {
		return nil, s.putObjectErr
	}

	// simulate some work with sleeping
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Millisecond):
		s.mu.Lock()
		defer s.mu.Unlock()
		s.putObjectCalled = true
		var err error
		s.putObjectBody, err = io.ReadAll(in.Body)
		if err != nil {
			return nil, err
		}
		return &s3.PutObjectOutput{}, nil
	}
}

func TestCreateBucketConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name     string
		regionIn string
		expected *s3types.CreateBucketConfiguration
	}{
		{
			name:     "special region",
			regionIn: "us-east-1",
			expected: nil,
		},
		{
			name:     "regular region",
			regionIn: "us-east-2",
			expected: &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraintUsEast2,
			},
		},
		{
			name:     "unknown region",
			regionIn: "unknown",
			expected: &s3types.CreateBucketConfiguration{
				LocationConstraint: "unknown",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateBucketConfiguration(tt.regionIn)
			require.Equal(t, tt.expected, got)
		})
	}
}
