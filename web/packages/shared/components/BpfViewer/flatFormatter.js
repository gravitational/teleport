/*
Copyright 2019 Gravitational, Inc.

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

import { formatCmd, formatNetwork, formatFile } from './formatters';

export default function flatFormatter(events, buffer) {
  for (var i = 0; i < events.length; i++) {
    const event = events[i];

    if (event.event === 'session.exec') {
      buffer.push(formatCmd(event));
    } else if (event.event === 'session.connect') {
      buffer.push(formatNetwork(event));
    } else if (event.event === 'session.open') {
      buffer.push(formatFile(event));
    }
  }
}
