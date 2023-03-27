/*
Copyright 2021 Gravitational, Inc.

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

package events

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// This is the max size of all the events we return when searching for events.
// 1 MiB was picked as a good middleground that's commonly used by the backends and is well below
// the max size limit of a response message. This may sometimes result in an even smaller size
// when serialized as protobuf but we have no good way to check so we go by raw from each backend.
const MaxEventBytesInResponse = 1024 * 1024

func estimateEventSize(event EventFields) (int, error) {
	data, err := utils.FastMarshal(event)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return len(data), nil
}
