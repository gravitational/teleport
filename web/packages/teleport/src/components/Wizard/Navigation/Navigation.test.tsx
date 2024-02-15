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
import React from 'react';
import { render, screen } from 'design/utils/testing';

import { Navigation } from './Navigation';

const steps = [{ title: 'first' }, { title: 'second' }, { title: 'third' }];

test('step 1/3', async () => {
  render(<Navigation views={steps} currentStep={0} />);

  const firstBullet = screen.getByTestId('bullet-active');
  expect(firstBullet).toHaveTextContent('');
  expect(firstBullet.parentElement).toHaveTextContent(/first/i);

  const uncheckedBullets = screen.getAllByTestId('bullet-default');
  expect(uncheckedBullets).toHaveLength(2);

  // second bullet
  expect(uncheckedBullets[0]).toHaveTextContent(/2/i);
  expect(uncheckedBullets[0].parentElement).toHaveTextContent(/second/i);

  // last bullet
  expect(uncheckedBullets[1]).toHaveTextContent(/3/i);
  expect(uncheckedBullets[1].parentElement).toHaveTextContent(/third/i);
});

test('step 2/3', async () => {
  render(<Navigation views={steps} currentStep={1} />);

  const firstBullet = screen.getByTestId('bullet-checked');
  expect(firstBullet).toHaveTextContent('');
  expect(firstBullet.parentElement).toHaveTextContent(/first/i);

  const secondBullet = screen.getByTestId('bullet-active');
  expect(secondBullet).toHaveTextContent('');
  expect(secondBullet.parentElement).toHaveTextContent(/second/i);

  const lastBullet = screen.getByTestId('bullet-default');
  expect(lastBullet).toHaveTextContent(/3/i);
  expect(lastBullet.parentElement).toHaveTextContent(/third/i);
});

test('step 3/3', async () => {
  render(<Navigation views={steps} currentStep={2} />);

  const checkedBullets = screen.getAllByTestId('bullet-checked');
  expect(checkedBullets).toHaveLength(2);

  // first bullet
  expect(checkedBullets[0]).toHaveTextContent('');
  expect(checkedBullets[0].parentElement).toHaveTextContent(/first/i);

  // second bullet
  expect(checkedBullets[1]).toHaveTextContent('');
  expect(checkedBullets[1].parentElement).toHaveTextContent(/second/i);

  // last bullet
  const lastBullet = screen.getByTestId('bullet-active');
  expect(lastBullet).toHaveTextContent('');
  expect(lastBullet.parentElement).toHaveTextContent(/third/i);
});
