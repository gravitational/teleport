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

import { Info } from 'design/Alert';
import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import cfg from 'teleport/config';
import {
  RequiredDiscoverProviders,
  resourceSpecAwsEc2Ssm,
} from 'teleport/Discover/Fixtures/fixtures';
import { AgentMeta, AutoDiscovery } from 'teleport/Discover/useDiscover';
import * as discoveryService from 'teleport/services/discovery/discovery';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import JoinTokenService from 'teleport/services/joinToken';
import { userEventService } from 'teleport/services/userEvent';
import TeleportContext from 'teleport/teleportContext';

import { DiscoveryConfigSsm } from './DiscoveryConfigSsm';

async function submitConfig() {
  fireEvent.click(screen.getByTestId('action-next'));
  await waitFor(() => {
    expect(screen.getByTestId('action-next')).toBeDisabled();
  });
  expect(discoveryService.createDiscoveryConfig).toHaveBeenCalledTimes(1);
}

const defaultIsCloud = cfg.isCloud;

async function completeForm() {
  expect(
    screen.getByText('Setup Discovery Config for Teleport Discovery Service')
  ).toBeInTheDocument();
  let selectEl = screen.getByLabelText(/aws region/i);
  fireEvent.focus(selectEl);
  fireEvent.keyDown(selectEl, { key: 'ArrowDown' });
  fireEvent.click(screen.getByText('us-east-2'));
  fireEvent.click(screen.getByTestId('region-next'));
  fireEvent.click(screen.getByTestId('script-next'));
  await waitFor(() => {
    expect(
      screen.getByText(/You can filter for EC2 instances by their tags/i)
    ).toBeInTheDocument();
  });
}

describe('DiscoveryConfigSsm', () => {
  beforeEach(() => {
    cfg.isCloud = true;

    jest
      .spyOn(userEventService, 'captureDiscoverEvent')
      .mockResolvedValue(undefined as never);
    jest.spyOn(discoveryService, 'createDiscoveryConfig').mockResolvedValue({
      name: '',
      discoveryGroup: '',
      aws: [],
    });

    jest
      .spyOn(JoinTokenService.prototype, 'fetchJoinTokenV2')
      .mockResolvedValue(tokenResp);
  });

  afterEach(() => {
    jest.clearAllMocks();
    jest.resetAllMocks();
    cfg.isCloud = defaultIsCloud;
  });

  test('calls with wildcard tag', async () => {
    render(<Component />);
    await completeForm();
    await submitConfig();
    const [[, configArg]] = (
      discoveryService.createDiscoveryConfig as jest.Mock
    ).mock.calls;
    expect(configArg.aws[0].tags).toEqual({ '*': ['*'] });
  });

  test('calls with entered tags and duplicate tags add to multivalue', async () => {
    render(<Component />);
    await completeForm();
    // add tags

    fireEvent.click(screen.getByText(/add a tag/i));
    const keyInput = screen.getByPlaceholderText('label key');
    const valInput = screen.getByPlaceholderText('label value');

    fireEvent.change(keyInput, { target: { value: 'asdf' } });
    fireEvent.change(valInput, { target: { value: 'fdsa' } });
    fireEvent.click(screen.getByText(/add another tag/i));

    const keyInput2 = screen.getAllByPlaceholderText('label key')[1];
    const valInput2 = screen.getAllByPlaceholderText('label value')[1];
    fireEvent.change(keyInput2, {
      target: { value: 'asdf' },
    });
    fireEvent.change(valInput2, {
      target: { value: 'ffff' },
    });

    await submitConfig();
    const [[, configArg]] = (
      discoveryService.createDiscoveryConfig as jest.Mock
    ).mock.calls;
    expect(configArg.aws[0].tags).toEqual({ asdf: ['fdsa', 'ffff'] });
  });
});

const Component = ({
  autoDiscovery = undefined,
}: {
  autoDiscovery?: AutoDiscovery;
}) => {
  const ctx = new TeleportContext();
  const agentMeta: AgentMeta = {
    resourceName: 'aws-console',
    agentMatcherLabels: [],
    awsIntegration: {
      kind: IntegrationKind.AwsOidc,
      name: 'some-oidc-name',
      resourceType: 'integration',
      spec: {
        roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
        issuerS3Bucket: '',
        issuerS3Prefix: '',
      },
      statusCode: IntegrationStatusCode.Running,
    },
    autoDiscovery,
  };

  return (
    <RequiredDiscoverProviders
      agentMeta={agentMeta}
      resourceSpec={resourceSpecAwsEc2Ssm}
      teleportCtx={ctx}
    >
      <Info>Devs: Click next to see next state</Info>
      <DiscoveryConfigSsm />
    </RequiredDiscoverProviders>
  );
};
const tokenResp = {
  allow: undefined,
  bot_name: undefined,
  content: undefined,
  expiry: null,
  expiryText: '',
  gcp: undefined,
  id: undefined,
  isStatic: undefined,
  method: undefined,
  internalResourceId: 'abc',
  roles: ['Application'],
  safeName: undefined,
  suggestedLabels: [],
};
