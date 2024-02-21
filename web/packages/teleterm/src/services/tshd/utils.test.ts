/**
 * Copyright 2024 Gravitational, Inc.
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

import { Timestamp } from 'google-protobuf/google/protobuf/timestamp_pb';

import { getProtoTimestamp } from './utils';

test('getProtoTimestamp null date returns null', () => {
  expect(getProtoTimestamp(null)).toBeFalsy();
  expect(getProtoTimestamp(undefined)).toBeFalsy();
});

test('getProtoTimestamp valid date returns proto timestamp', () => {
  const date = new Date('2024-02-16T03:00:00.156944Z');

  const protoTimestamp = new Timestamp();
  protoTimestamp.fromDate(date);

  expect(getProtoTimestamp(date)).toEqual(protoTimestamp);
});
