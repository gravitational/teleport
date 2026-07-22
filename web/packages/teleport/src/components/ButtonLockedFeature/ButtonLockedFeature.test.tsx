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

import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { CtaEvent, userEventService } from 'teleport/services/userEvent';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { ButtonLockedFeature } from './ButtonLockedFeature';

const defaultIsEnterpriseFlag = cfg.isEnterprise;

describe('buttonLockedFeature', () => {
  afterEach(() => {
    jest.resetAllMocks();

    cfg.isEnterprise = defaultIsEnterpriseFlag;
  });

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

  test('it has upgrade-community href for community edition', () => {
    const version = ctx.storeUser.state.cluster.authVersion;
    cfg.isEnterprise = false;
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

  test('it has upgrade-igs href for Enterprise + IGS Plan', () => {
    const version = ctx.storeUser.state.cluster.authVersion;
    cfg.isEnterprise = true;

    renderWithContext(
      <ButtonLockedFeature noIcon={true}>text</ButtonLockedFeature>
    );
    expect(screen.getByText('text').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-igs?e_${version}&utm_campaign=CTA_UNSPECIFIED`
    );

    renderWithContext(
      <ButtonLockedFeature noIcon={true} event={CtaEvent.CTA_ACCESS_REQUESTS}>
        othertext
      </ButtonLockedFeature>
    );
    expect(screen.getByText('othertext').closest('a')).toHaveAttribute(
      'href',
      `https://goteleport.com/r/upgrade-igs?e_${version}&utm_campaign=${
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
      cfg.isEnterprise = true;
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
