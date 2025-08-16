/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { screen } from '@testing-library/react';

import { render, userEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import {
  IamIntegration,
  parseOutput,
} from 'teleport/Integrations/Enroll/AwsConsole/IamIntegration/IamIntegration';
import { makeAwsOidcStatusContextState } from 'teleport/Integrations/status/AwsOidc/testHelpers/makeAwsOidcStatusContextState';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { integrationService } from 'teleport/services/integrations';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

test('flows through roles anywhere IAM setup', async () => {
  const user = userEvent.setup();

  const pingSpy = jest
    .spyOn(integrationService, 'awsRolesAnywherePing')
    .mockResolvedValue({} as any);

  render(
    <ContextProvider ctx={createTeleportContext()}>
      <QueryClientProvider client={queryClient}>
        <MockAwsOidcStatusProvider
          value={makeAwsOidcStatusContextState()}
          path=""
        >
          <IamIntegration />
        </MockAwsOidcStatusProvider>
      </QueryClientProvider>
    </ContextProvider>
  );

  expect(
    screen.getByText('Step 1: Name your Teleport Integration')
  ).toBeInTheDocument();
  expect(
    screen.queryByText('Step 2: Create Roles Anywhere Trust Anchor')
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Next: Configure Access' })
  ).toBeDisabled();

  await user.type(
    screen.getByLabelText('Integration Name'),
    'some-integration-name'
  );
  await user.click(screen.getByRole('button', { name: 'Generate Command' }));

  expect(
    screen.getByText('Step 2: Create Roles Anywhere Trust Anchor')
  ).toBeInTheDocument();
  expect(
    screen.getByText('Step 3: Create and Sync the Integration Profile and Role')
  ).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Test Configuration' })
  ).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Next: Configure Access' })
  ).toBeDisabled();

  await user.type(
    screen.getByLabelText('Trust Anchor, Profile and Role ARNs'),
    'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo\n' +
      'arn:aws:rolesanywhere:eu-west-2:123456789012:profile/bar\n' +
      'arn:aws:iam::123456789012:role/baz'
  );

  expect(
    screen.getByRole('button', { name: 'Test Configuration' })
  ).toBeEnabled();
  await user.click(screen.getByRole('button', { name: 'Test Configuration' }));
  expect(pingSpy).toHaveBeenCalledTimes(1);
  expect(pingSpy).toHaveBeenCalledWith({
    integrationName: 'some-integration-name',
    syncProfileArn: 'arn:aws:rolesanywhere:eu-west-2:123456789012:profile/bar',
    syncRoleArn: 'arn:aws:iam::123456789012:role/baz',
    trustAnchorArn:
      'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/foo',
  });

  expect(
    screen.getByRole('button', { name: 'Next: Configure Access' })
  ).toBeEnabled();
});

describe('parseOutput', () => {
  const excess = `
  4. Create a Roles Anywhere Profile in AWS IAM for your Teleport cluster.
CreateRolesAnywhereProfileProvider: {
    "Name": "RAProfileFromCLI",
    "RoleArns": [
        "arn:aws:iam::123456789012:role/ra-role"
    ],

Copy and paste the following values to Teleport UI

=================================================
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/ra-role
=================================================

2025-05-15T16:30:21.683+01:00 INFO  Success! operation:awsra-trust-anchor provisioning/operations.go:190
`;
  const topExcess = `

=================================================
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/ra-role
`;
  const bottomExcess = `
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/ra-role
=================================================
`;
  const perfect = `
arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000
arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000
arn:aws:iam::123456789012:role/ra-role
`;
  const empty = '';

  const valid = {
    trustAnchorArn:
      'arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/00000000-0000-0000-0000-000000000000',
    syncProfileArn:
      'arn:aws:rolesanywhere:eu-west-2:123456789012:profile/00000000-0000-0000-0000-000000000000',
    syncRoleArn: 'arn:aws:iam::123456789012:role/ra-role',
  };

  test.each`
    name                          | input           | expected
    ${'valid excess copy'}        | ${excess}       | ${valid}
    ${'valid excess top copy'}    | ${topExcess}    | ${valid}
    ${'valid excess bottom copy'} | ${bottomExcess} | ${valid}
    ${'valid perfect copy'}       | ${perfect}      | ${valid}
    ${'invalid empty'}            | ${empty}        | ${undefined}
  `(`parseOutput $name`, ({ input, expected }) => {
    const values = parseOutput(input);
    expect(values).toEqual(expected);
  });
});
