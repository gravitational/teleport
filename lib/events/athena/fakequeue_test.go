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

package athena

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	snsTypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// fakeQueue is used to fake SNS+SQS combination on AWS.
type fakeQueue struct {
	// publishErrors is chain of error returns on Publish method.
	// Errors are returned from start to end and removed, one-by-one, on each
	// invocation of the Publish method.
	// If the slice is empty, Publish runs normally.
	publishErrors []error
	mu            sync.Mutex
	msgs          []fakeQueueMessage
}

type fakeQueueMessage struct {
	payload    string
	attributes map[string]snsTypes.MessageAttributeValue
}

func newFakeQueue() *fakeQueue {
	return &fakeQueue{}
}

func (f *fakeQueue) Publish(ctx context.Context, params *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.publishErrors) > 0 {
		err := f.publishErrors[0]
		f.publishErrors = f.publishErrors[1:]
		return nil, err
	}
	f.msgs = append(f.msgs, fakeQueueMessage{
		payload:    *params.Message,
		attributes: params.MessageAttributes,
	})
	return nil, nil
}

func (f *fakeQueue) getMessages() []fakeQueueMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	batchSize := 10
	if len(f.msgs) < 1 {
		return nil
	}
	if len(f.msgs) < batchSize {
		batchSize = len(f.msgs)
	}
	items := f.msgs[:batchSize]
	f.msgs = f.msgs[batchSize:]
	return items
}
