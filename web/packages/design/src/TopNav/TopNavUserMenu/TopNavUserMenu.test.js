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

import React from 'react';

import { render, fireEvent } from 'design/utils/testing';

import TopNavUserMenu from './TopNavUserMenu';

test('onShow and onClose fn prop is respected', () => {
  const onShow = jest.fn();
  const onClose = jest.fn();
  const { container, rerender } = render(
    <TopNavUserMenu open={false} onShow={onShow} onClose={onClose} />
  );

  fireEvent.click(container.firstChild);
  expect(onShow).toHaveBeenCalledTimes(1);

  rerender(<TopNavUserMenu open={true} onShow={onShow} onClose={onClose} />);

  fireEvent.keyDown(container, { key: 'Escape' });
  expect(onClose).toHaveBeenCalledTimes(1);
});
