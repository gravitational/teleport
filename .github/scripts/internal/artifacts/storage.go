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

	"cloud.google.com/go/storage"
)

// objectAdaptor wraps a live GCB storage handle in our minimal storage
// interface
type objectAdaptor struct{ *storage.ObjectHandle }

func (adaptor objectAdaptor) NewWriter(ctx context.Context) io.WriteCloser {
	return adaptor.ObjectHandle.NewWriter(ctx)
}

// objectAdaptor wraps a live GCB storage handle in our minimal bucket
// interface
type bucketAdaptor struct{ *storage.BucketHandle }

func (adaptor bucketAdaptor) Object(name string) objectHandle {
	return objectAdaptor{adaptor.BucketHandle.Object(name)}
}
