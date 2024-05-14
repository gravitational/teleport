/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { ContextProvider } from 'teleport/index';
import cfg from 'teleport/config';
import { clusters } from 'teleport/Clusters/fixtures';

import { storageService } from 'teleport/services/storageService';

import { getAcl } from 'teleport/mocks/contexts';

import { ExternalAuditStorageCta } from './ExternalAuditStorageCta';

describe('externalAuditStorageCta', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  type SetupParams = {
    isCloud: boolean;
    lockedFeature: boolean;
    hasPermission;
  };
  const setup = ({ isCloud, lockedFeature, hasPermission }: SetupParams) => {
    const noPermAcl = { customAcl: getAcl({ noAccess: true }) };
    const ctx = createTeleportContext(hasPermission ? null : noPermAcl);
    ctx.storeUser.setState({
      username: 'joe@example.com',
      cluster: clusters[0],
    });

    cfg.isCloud = isCloud;
    cfg.externalAuditStorage = lockedFeature;

    jest
      .spyOn(storageService, 'getExternalAuditStorageCtaDisabled')
      .mockReturnValue(false);

    const { container } = render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <ExternalAuditStorageCta />
        </ContextProvider>
      </MemoryRouter>
    );

    return { container, ctx };
  };

  test('renders the CTA', () => {
    setup({ isCloud: true, lockedFeature: false, hasPermission: true });
    expect(screen.getByText(/External Audit Storage/)).toBeInTheDocument();
    expect(screen.getByText(/Connect your AWS storage/)).toBeEnabled();
  });

  test('renders nothing on cfg.isCloud=false', () => {
    const { container } = setup({
      isCloud: false,
      lockedFeature: true,
      hasPermission: true,
    });
    expect(container).toBeEmptyDOMElement();
  });

  test('renders button based on lockedFeatures', () => {
    setup({ isCloud: true, lockedFeature: false, hasPermission: true });
    expect(screen.getByText(/Connect your AWS storage/)).toBeInTheDocument();
    expect(screen.getByText(/Connect your AWS storage/)).toBeEnabled();

    setup({ isCloud: true, lockedFeature: true, hasPermission: true });
    expect(screen.getByText(/Contact Sales/)).toBeInTheDocument();
  });

  test('renders disabled button if no permissions', () => {
    setup({ isCloud: true, lockedFeature: false, hasPermission: false });
    expect(screen.getByText(/Connect your AWS storage/)).toBeInTheDocument();
    // eslint wants us to use `toBeDisabled` instead of toHaveAttribute
    // but this causes the test to fail, since the button is rendered as an anchor tag
    // eslint-disable-next-line
    expect(screen.getByText(/Connect your AWS storage/)).toHaveAttribute(
      'disabled'
    );
  });
});
