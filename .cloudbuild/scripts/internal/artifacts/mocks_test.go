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

package artifacts

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"
)

type mockBucket struct {
	mock.Mock
}

func (b *mockBucket) Object(name string) objectHandle {
	args := b.Called(name)
	return args.Get(0).(objectHandle)
}

type mockStorageObject struct {
	mock.Mock
}

func (obj *mockStorageObject) NewWriter(ctx context.Context) io.WriteCloser {
	args := obj.Called(ctx)
	return args.Get(0).(io.WriteCloser)
}

type closeWrapper struct {
	io.Writer
	closeError error
}

func (w *closeWrapper) Close() error {
	return w.closeError
}

type mockWriter struct {
	mock.Mock
}

func (w *mockWriter) Write(data []byte) (int, error) {
	args := w.Called(data)

	// If no explicit return value was set in the mock then return as if we
	// have written the whole buffer. The testify mock library makes it hard
	// to return values calculated at execution time (as opposed to at test
	// construction), so we add this shim to make writing tests with it a
	// bit easier.
	if len(args) == 0 {
		return len(data), nil
	}

	return args.Int(0), args.Error(1)
}

func (w *mockWriter) Close() error {
	args := w.Called()
	return args.Error(0)
}
