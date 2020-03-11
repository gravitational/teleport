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

  fb.setFailed(new Error('errMsg'));
  expect(fb.state.statusText).toBe('errMsg');
  expect(fb.isFailed()).toBe(true);

  fb.setProcessing();
  expect(fb.isProcessing()).toBe(true);

  fb.setReady();
  expect(fb.isReady()).toBe(true);

  fb.setDisabled();
  expect(fb.isDisabled()).toBe(true);
});

test('class Activator', () => {
  const loadable1 = {
    onload: jest.fn(),
  };

  const loadable2 = {
    onload: jest.fn(),
  };

  const ctx = {
    name: 'sam',
  };

  const activator = new Activator<any>([loadable1, loadable2]);
  activator.onload(ctx);
  expect(loadable1.onload).toHaveBeenCalledWith(ctx);
  expect(loadable1.onload).toHaveBeenCalledTimes(1);
  expect(loadable2.onload).toHaveBeenCalledTimes(1);
});
