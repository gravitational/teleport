/**
 * Copyright 2020 Gravitational, Inc.
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

import FeatureBase, { Activator } from './featureBase';

jest.mock('./logger', () => {
  const mockLogger = {
    error: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

test('class FeatureBase: setters and boolean getters', () => {
  const fb = new FeatureBase();

  // default states
  expect(fb.state.statusText).toBe('');
  expect(fb.state.status).toBe('uninitialized');
  expect(fb.isProcessing()).toBe(false);
  expect(fb.isReady()).toBe(false);
  expect(fb.isDisabled()).toBe(false);
  expect(fb.isFailed()).toBe(false);

  fb.setFailed({ message: 'errMsg' });
  expect(fb.state.statusText).toBe('errMsg');
  expect(fb.state.status).toBe('failed');
  expect(fb.isFailed()).toBe(true);

  fb.setProcessing();
  expect(fb.state.statusText).toBe('');
  expect(fb.state.status).toBe('processing');
  expect(fb.isProcessing()).toBe(true);

  fb.setReady();
  expect(fb.state.statusText).toBe('');
  expect(fb.state.status).toBe('ready');
  expect(fb.isReady()).toBe(true);

  fb.setDisabled();
  expect(fb.state.statusText).toBe('');
  expect(fb.state.status).toBe('disabled');
  expect(fb.isDisabled()).toBe(true);
});

test('class Activator', () => {
  const featureA = new FeatureBase();
  const featureB = new FeatureBase();
  const activator = new Activator([featureA, featureB]);

  jest.spyOn(featureA, 'onload');
  jest.spyOn(featureB, 'onload');

  activator.onload({});

  expect(featureA.onload).toHaveBeenCalledTimes(1);
  expect(featureB.onload).toHaveBeenCalledTimes(1);
});
