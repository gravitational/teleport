/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export default function parseError(json) {
  let msg = '';

  if (json && json.error) {
    msg = json.error.message;
  } else if (json && json.message) {
    msg = json.message;
  } else if (json.responseText) {
    msg = json.responseText;
  }
  return msg;
}

export class ApiError extends Error {
  response: Response;

  constructor(message, response: Response) {
    message = message || 'Unknown error';
    super(message);
    this.response = response;
    this.name = 'ApiError';
  }
}
