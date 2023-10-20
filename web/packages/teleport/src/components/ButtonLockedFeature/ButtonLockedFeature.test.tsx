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
import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'teleport/config';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { CtaEvent, userEventService } from 'teleport/services/userEvent';

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

  test('it has upgrade-team href for Team Plan', () => {
    const version = ctx.storeUser.state.cluster.authVersion;
    cfg.isUsageBasedBilling = true;

    renderWithContext(
      <ButtonLockedFeature noIcon={true}>text</ButtonLockedFeature>
    );
    expect(screen.getByText('text').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-team?e_${version}&utm_campaign=CTA_UNSPECIFIED`
    );

    renderWithContext(
      <ButtonLockedFeature noIcon={true} event={CtaEvent.CTA_ACCESS_REQUESTS}>
        othertext
      </ButtonLockedFeature>
    );
    expect(screen.getByText('othertext').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-team?e_${version}&utm_campaign=${
        CtaEvent[CtaEvent.CTA_ACCESS_REQUESTS]
      }`
    );
  });

  test('it has upgrade-community href for community edition', () => {
    const version = ctx.storeUser.state.cluster.authVersion;
    cfg.isUsageBasedBilling = false;
    renderWithContext(
      <ButtonLockedFeature noIcon={true}>text</ButtonLockedFeature>,
      {
        enterprise: false,
      }
    );
    expect(screen.getByText('text').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-community?${version}&utm_campaign=CTA_UNSPECIFIED`
    );

    renderWithContext(
      <ButtonLockedFeature noIcon={true} event={CtaEvent.CTA_ACCESS_REQUESTS}>
        othertext
      </ButtonLockedFeature>,
      {
        enterprise: false,
      }
    );
    expect(screen.getByText('othertext').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-community?${version}&utm_campaign=${
        CtaEvent[CtaEvent.CTA_ACCESS_REQUESTS]
      }`
    );
  });

  describe('userEventService', () => {
    beforeEach(() => {
      jest.spyOn(userEventService, 'captureCtaEvent');
    });

    afterEach(() => {
      jest.resetAllMocks();
      jest.clearAllMocks();
    });

    test('does not invoke userEventService for oss', async () => {
      renderWithContext(<ButtonLockedFeature>content</ButtonLockedFeature>, {
        enterprise: false,
      });

      await userEvent.click(screen.getByText('content').closest('a'));
      expect(userEventService.captureCtaEvent).not.toHaveBeenCalled();
    });

    test('invokes userEventService for enterprise', async () => {
      renderWithContext(
        <ButtonLockedFeature event={CtaEvent.CTA_ACCESS_REQUESTS}>
          content
        </ButtonLockedFeature>
      );

      await userEvent.click(screen.getByText('content').closest('a'));
      expect(userEventService.captureCtaEvent).toHaveBeenCalledWith(
        CtaEvent.CTA_ACCESS_REQUESTS
      );
    });
  });
});

const ctx = createTeleportContext();

type renderProps = {
  enterprise?: boolean;
};

function renderWithContext(
  ui: React.ReactElement,
  { enterprise = true }: renderProps = {}
) {
  ctx.isEnterprise = enterprise;

  return render(
    <TeleportContextProvider ctx={ctx}>{ui}</TeleportContextProvider>
  );
}
