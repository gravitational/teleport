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

import TeleportContext from 'teleport/teleportContext';
import { ContextProvider } from 'teleport/index';
import cfg from 'teleport/config';
import { clusters } from 'teleport/Clusters/fixtures';

import { storageService } from 'teleport/services/storageService';

import { ExternalAuditStorageCta } from './ExternalAuditStorageCta';

describe('externalAuditStorageCta', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  const setup = (isCloud: boolean, losckedFeature: boolean) => {
    const ctx = new TeleportContext();
    ctx.storeUser.setState({
      username: 'joe@example.com',
      cluster: clusters[0],
    });

    cfg.isCloud = isCloud;
    ctx.lockedFeatures.externalCloudAudit = losckedFeature;

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
    setup(true, false);
    expect(screen.getByText(/External Audit Storage/)).toBeInTheDocument();
  });

  test('renders nothing on cfg.isCloud=false', () => {
    const { container } = setup(false, true);
    expect(container).toBeEmptyDOMElement();
  });

  test('renders button based on lockedFeatures', () => {
    setup(true, false);
    expect(screen.getByText(/Connect your AWS storage/)).toBeInTheDocument();

    setup(true, true);
    expect(screen.getByText(/Contact Sales/)).toBeInTheDocument();
  });
});
