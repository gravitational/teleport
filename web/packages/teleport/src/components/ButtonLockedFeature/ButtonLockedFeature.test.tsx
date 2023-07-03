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
import { render, screen } from 'design/utils/testing';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { CtaEvent } from 'teleport/services/userEvent';

import { ButtonLockedFeature } from './ButtonLockedFeature';

describe('buttonLockedFeature', () => {
  test('renders the children', () => {
    const content = "this is the button's content";
    renderWithContext(<ButtonLockedFeature>{content}</ButtonLockedFeature>);
    expect(screen.getByText(content)).toBeInTheDocument();
  });

  test('it renders the icon by default', () => {
    renderWithContext(<ButtonLockedFeature>text</ButtonLockedFeature>);
    expect(screen.getByText('text')).toBeInTheDocument();
    expect(screen.getByTestId('locked-icon')).toBeInTheDocument();
  });

  test('it renders without the icon when noIcon=true', () => {
    renderWithContext(
      <ButtonLockedFeature noIcon={true}>text</ButtonLockedFeature>
    );
    expect(screen.getByText('text')).toBeInTheDocument();
    expect(screen.queryByTestId('locked-icon')).not.toBeInTheDocument();
  });

  test('it has the proper href', () => {
    const version = ctx.storeUser.state.cluster.authVersion;

    renderWithContext(
      <ButtonLockedFeature noIcon={true}>text</ButtonLockedFeature>
    );
    expect(screen.getByText('text').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-team?${version}&utm_campaign=undefined`
    );

    renderWithContext(
      <ButtonLockedFeature noIcon={true} event={CtaEvent.CTA_ACCESS_REQUESTS}>
        othertext
      </ButtonLockedFeature>
    );
    expect(screen.getByText('othertext').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-team?${version}&utm_campaign=${
        CtaEvent[CtaEvent.CTA_ACCESS_REQUESTS]
      }`
    );
  });
});

const ctx = createTeleportContext();
function renderWithContext(ui: React.ReactElement) {
  return render(
    <TeleportContextProvider ctx={ctx}>{ui}</TeleportContextProvider>
  );
}
