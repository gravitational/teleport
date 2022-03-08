import React from 'react';
import { fireEvent, render, screen } from 'design/utils/testing';
import Automatically, { createAppBashCommand } from './Automatically';

test('render command only after form submit', async () => {
  const token = 'token';
  render(
    <Automatically
      token={token}
      attempt={{ status: 'success' }}
      onClose={() => {}}
      onCreate={() => Promise.resolve(true)}
      expires=""
    />
  );

  // initially, should not show the command
  let cmd = createAppBashCommand(token, '', '');
  expect(screen.queryByText(cmd)).toBeNull();

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
  cmd = createAppBashCommand(token, 'app-name', 'https://gravitational.com');
  expect(screen.queryByText(cmd)).not.toBeNull();
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
