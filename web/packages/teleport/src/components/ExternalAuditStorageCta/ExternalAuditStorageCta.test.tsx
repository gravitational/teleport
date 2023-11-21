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
import { screen } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';

import { ExternalAuditStorageCta } from './ExternalAuditStorageCta';

function render(children) {
  return renderWithElementsAndContext(<MemoryRouter>{children}</MemoryRouter>);
}

describe('externalAuditStorageCta', () => {
  test('renders the CTA', () => {
    render(
      <ExternalAuditStorageCta
        isEnabled={true}
        showCta={true}
        onDismiss={() => null}
      />
    );
    expect(screen.getByText(/External Audit Storage/)).toBeInTheDocument();
  });

  test('renders nothing on showCta=false', () => {
    const { container } = render(
      <ExternalAuditStorageCta
        isEnabled={true}
        showCta={false}
        onDismiss={() => null}
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  test('renders button based on isEnabled', () => {
    render(
      <ExternalAuditStorageCta
        isEnabled={true}
        showCta={true}
        onDismiss={() => null}
      />
    );
    expect(screen.getByText(/Connect your AWS storage/)).toBeInTheDocument();

    render(
      <ExternalAuditStorageCta
        isEnabled={false}
        showCta={true}
        onDismiss={() => null}
      />
    );
    expect(screen.getByText(/Contact Sales/)).toBeInTheDocument();
  });
});
