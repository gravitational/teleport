/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { ContextProvider } from 'teleport/index';
import cfg from 'teleport/config';
import { clusters } from 'teleport/Clusters/fixtures';
import localStorage from 'teleport/services/localStorage';

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
    ctx.lockedFeatures.externalCloudAudit = lockedFeature;

    jest
      .spyOn(localStorage, 'getExternalAuditStorageCtaDisabled')
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
