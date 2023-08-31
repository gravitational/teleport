// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
