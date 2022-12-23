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

import { BroadcastChannel } from 'broadcast-channel';

import { WebSession } from 'teleport/services/websession';

// Creates and returns a mocked webSession to be used in unit tests.
export function getMockWebSession() {
  jest.mock('broadcast-channel');

  // The 'broadcast-channel' module is used for mocking in unit tests,
  // however its type differs slightly from the native BroadcastChannel
  // used in the app. This type conversion to unknown and then to the native
  // BroadcastChannel type is done in order for the WebSession constructor
  // to be able to accept this slightly different type.
  const mockBcBroadcaster = new BroadcastChannel(
    'test'
  ) as unknown as globalThis.BroadcastChannel;
  const mockBcReceiver = new BroadcastChannel(
    'test'
  ) as unknown as globalThis.BroadcastChannel;

  const mockWebSession = new WebSession(mockBcBroadcaster, mockBcReceiver);
  mockBcReceiver.onmessage = () => {};

  return mockWebSession;
}
