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

export const isAbortError = (err: any): boolean => {
  // handles Web UI abort error
  if (
    (err instanceof DOMException && err.name === 'AbortError') ||
    (err.cause && isAbortError(err.cause))
  ) {
    return true;
  }

  // handles Connect abort error (specifically gRPC cancel error)
  // the error has only the message field that contains the following string:
  // '1 CANCELLED: Cancelled on client'
  return err instanceof Error && err.message?.includes('CANCELLED');
};
