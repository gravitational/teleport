/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { isAbortError } from 'shared/utils/abortError';

describe.each([
  ['DOMException', newDOMAbortError],
  ['ApiError', newApiAbortError],
  ['gRPC Error', newGrpcAbortError],
])('for error type %s', (_, ErrorType) => {
  it('is abort error', async () => {
    expect(isAbortError(ErrorType())).toBe(true);
  });
});

function newDOMAbortError() {
  return new DOMException('Aborted', 'AbortError');
}

// mimics ApiError
function newApiAbortError() {
  return new Error('The user aborted a request', {
    cause: newDOMAbortError(),
  });
}

function newGrpcAbortError() {
  return new Error('1 CANCELLED: Cancelled on client');
}
