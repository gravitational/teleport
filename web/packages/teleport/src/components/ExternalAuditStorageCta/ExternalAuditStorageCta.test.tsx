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
