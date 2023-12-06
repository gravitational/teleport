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
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { getAcl } from 'teleport/mocks/contexts';
import { ApiError } from 'teleport/services/api/parseError';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { accessManagementService } from 'e-teleport/services/accessmanagement';
import TeleportEContext from 'e-teleport/teleportContextE';

import { AccessLists } from './AccessLists';

const defaultIsTeamFlag = cfg.isTeam;
const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultIgsFlag = cfg.isIgsEnabled;
const defaultIsUsageBased = cfg.isUsageBasedBilling;

describe('upsell links', () => {
  const ctx = createTeleportContextE();

  beforeEach(() => {
    cfg.isEnterprise = true;
    cfg.isUsageBasedBilling = true;

    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([]);
  });

  afterEach(() => {
    jest.resetAllMocks();

    cfg.isTeam = defaultIsTeamFlag;
    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.isIgsEnabled = defaultIgsFlag;
    cfg.isUsageBasedBilling = defaultIsUsageBased;
  });

  test('no access should not render cta', async () => {
    const error = new ApiError('', { status: 403 } as Response);

    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockRejectedValue(error);

    ecfg.oss.isIgsEnabled = true;
    ecfg.oss.isTeam = false;

    const ctx = createTeleportContextE({
      customAcl: getAcl({ noAccess: true }),
    });

    renderComponent(ctx);

    await screen.findByText(/can only be viewed by their owners/i);
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();

    // eslint-disable-next-line jest-dom/prefer-enabled-disabled
    expect(screen.getByText(/create new access list/i)).toHaveAttribute(
      'disabled'
    );
  });

  test('render limited preview for non-usage based', async () => {
    ecfg.oss.isUsageBasedBilling = false;

    renderComponent(ctx);

    await screen.findByText(/preview and will limit/i);
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
  });

  test('eub with igs enabled renders no cta', async () => {
    ecfg.oss.isIgsEnabled = true;
    ecfg.oss.isTeam = false;

    renderComponent(ctx);

    await screen.findByText(/create your first access list/i);
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
  });

  test('team renders cta', async () => {
    ecfg.oss.isIgsEnabled = false;
    ecfg.oss.isTeam = true;

    renderComponent(ctx);

    await screen.findByText(/create your first access list/i);
    const link = screen.getByText(/contact sales/i);
    expect(link).toHaveAttribute(
      'href',
      expect.stringMatching(/upgrade-team/i)
    );
  });

  test('eub WITHOUT igs renders cta', async () => {
    ecfg.oss.isIgsEnabled = false;
    ecfg.oss.isTeam = false;

    renderComponent(ctx);

    await screen.findByText(/create your first access list/i);
    const link = screen.getByText(/contact sales/i);
    expect(link).toHaveAttribute('href', expect.stringMatching(/upgrade-igs/i));
  });
});

function renderComponent(ctx: TeleportEContext) {
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AccessLists />
      </ContextProvider>
    </MemoryRouter>
  );
}
