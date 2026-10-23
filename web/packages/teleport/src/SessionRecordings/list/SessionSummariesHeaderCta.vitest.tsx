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

import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { render, screen, userEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { SessionSummariesHeaderCta } from './SessionSummariesHeaderCta';

const TRIGGER = 'Summarize Sessions with AI';
const DOCS_URL =
  'https://goteleport.com/docs/identity-security/session-summaries/';

let isEnterprise: boolean;
let isCloud: boolean;

beforeEach(() => {
  isEnterprise = cfg.edition === 'ent';
  isCloud = cfg.isCloud;
});

afterEach(() => {
  cfg.edition = isEnterprise ? 'ent' : 'oss';
  cfg.isCloud = isCloud;
});

function renderCta(edition: { isEnterprise: boolean; isCloud?: boolean }) {
  cfg.edition = edition.isEnterprise ? 'ent' : 'oss';
  cfg.isCloud = edition.isCloud ?? false;

  return render(
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContext()}>
        <SessionSummariesHeaderCta />
      </ContextProvider>
    </MemoryRouter>
  );
}

async function openPopover() {
  const user = userEvent.setup();

  await user.click(screen.getByRole('button', { name: TRIGGER }));

  return user;
}

describe('the header promotion', () => {
  it('asks nothing until the trigger is used', () => {
    renderCta({ isEnterprise: false });

    expect(screen.getByRole('button', { name: TRIGGER })).toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: 'Learn more' })
    ).not.toBeInTheDocument();
  });

  it('asks Community users to upgrade the edition', async () => {
    renderCta({ isEnterprise: false });
    await openPopover();

    const action = await screen.findByRole('link', {
      name: 'Unlock Session Summaries',
    });

    // Community has no entitlements, so this is the community upgrade page.
    expect(action).toHaveAttribute(
      'href',
      expect.stringContaining('upgrade-community')
    );
  });

  it('asks Enterprise users to unlock the feature, which they may already own the edition for', async () => {
    renderCta({ isEnterprise: true });
    await openPopover();

    const action = await screen.findByRole('link', {
      name: 'Unlock Session Summaries',
    });

    expect(action).toHaveAttribute(
      'href',
      expect.stringContaining('utm_campaign=CTA_IDENTITY_SECURITY')
    );
  });

  it('asks a Cloud tenant to trial it, matching the recording panel', async () => {
    renderCta({ isEnterprise: true, isCloud: true });
    await openPopover();

    expect(
      await screen.findByRole('link', { name: 'Start a trial' })
    ).toBeInTheDocument();
  });

  it('demotes the docs to a secondary link in both editions', async () => {
    renderCta({ isEnterprise: false });
    await openPopover();

    expect(
      await screen.findByRole('link', { name: 'Learn more' })
    ).toHaveAttribute('href', DOCS_URL);
  });

  it('closes the popover on Escape', async () => {
    renderCta({ isEnterprise: false });
    const user = await openPopover();

    await screen.findByRole('link', { name: 'Learn more' });

    await user.keyboard('{Escape}');

    expect(
      screen.queryByRole('link', { name: 'Learn more' })
    ).not.toBeInTheDocument();
  });
});
