/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import {
  fireEvent,
  render,
  screen,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import { MemoryRouter } from 'react-router';

import { userEventService } from 'teleport/services/userEvent';

import { AwsOidc } from './AwsOidc';

test('render', async () => {
  jest
    .spyOn(userEventService, 'captureIntegrationEnrollEvent')
    .mockImplementation();
  render(
    <MemoryRouter>
      <AwsOidc />
    </MemoryRouter>
  );

  expect(screen.getByText(/Set up your AWS account/i)).toBeInTheDocument();
  expect(
    screen.getByLabelText(/Give this AWS integration a name/i)
  ).toBeInTheDocument();
  expect(
    screen.getByLabelText(
      /Give a name for an AWS IAM role this integration will create/i
    )
  ).toBeInTheDocument();
});

test('generate command', async () => {
  const user = userEvent.setup({ delay: null });
  jest
    .spyOn(userEventService, 'captureIntegrationEnrollEvent')
    .mockImplementation();

  window.prompt = jest.fn();

  render(
    <MemoryRouter>
      <AwsOidc />
    </MemoryRouter>
  );

  const pluginConfig = {
    name: 'integration-name',
    roleName: 'integration-role-name',
  };

  expect(screen.getByText(/Set up your AWS account/i)).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText(/Give this AWS integration a name/i), {
    target: { value: pluginConfig.name },
  });
  fireEvent.change(
    screen.getByLabelText(
      /Give a name for an AWS IAM role this integration will create/i
    ),
    {
      target: { value: pluginConfig.roleName },
    }
  );

  fireEvent.click(screen.getByRole('button', { name: /Generate Command/i }));

  const commandBoxEl = screen.getByText(/AWS CloudShell/i, { exact: false });
  await waitFor(() => {
    expect(commandBoxEl).toBeInTheDocument();
  });

  // the first element found shows AWS tags added by OIDC integraiton.
  // second element is the command copy box.
  await user.click(screen.getAllByTestId('btn-copy')[1]);
  const clipboardText = await navigator.clipboard.readText();
  expect(clipboardText).toContain(`integrationName=${pluginConfig.name}`);
  expect(clipboardText).toContain(`role=${pluginConfig.roleName}`);
});
