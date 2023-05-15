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

import React from 'react';
import { fireEvent, render, screen } from 'design/utils/testing';

import { Automatically, createAppBashCommand } from './Automatically';

test('render command only after form submit', async () => {
  const token = { id: 'token', expiryText: '', expiry: null };
  render(
    <Automatically
      token={token}
      attempt={{ status: 'success' }}
      onClose={() => {}}
      onCreate={() => Promise.resolve(true)}
    />
  );

  // initially, should not show the command
  let cmd = createAppBashCommand(token.id, '', '');
  expect(screen.queryByText(cmd)).not.toBeInTheDocument();

  // set app name
  const appNameInput = screen.getByPlaceholderText('jenkins');
  fireEvent.change(appNameInput, { target: { value: 'app-name' } });

  // set app url
  const appUriInput = screen.getByPlaceholderText('https://localhost:4000');
  fireEvent.change(appUriInput, {
    target: { value: 'https://gravitational.com' },
  });

  // click button
  screen.getByRole('button', { name: /Generate Script/i }).click();

  // after form submission should show the command
  cmd = createAppBashCommand(token.id, 'app-name', 'https://gravitational.com');
  expect(screen.getByText(cmd)).toBeInTheDocument();
});

test('app bash command encoding', () => {
  const token = '86';
  const appName = 'jenkins';
  const appUri = `http://myapp/test?b='d'&a="1"&c=|`;

  const cmd = createAppBashCommand(token, appName, appUri);
  expect(cmd).toBe(
    `sudo bash -c "$(curl -fsSL 'http://localhost/scripts/86/install-app.sh?name=jenkins&uri=http%3A%2F%2Fmyapp%2Ftest%3Fb%3D%27d%27%26a%3D%221%22%26c%3D%7C')"`
  );
});
