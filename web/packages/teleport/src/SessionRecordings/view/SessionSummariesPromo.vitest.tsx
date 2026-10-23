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
import { beforeEach, describe, expect, it } from 'vitest';

import { act, render, screen, userEvent, waitFor } from 'design/utils/testing';

import { SessionSummariesPromo } from './SessionSummariesPromo';

const DOCS_URL =
  'https://goteleport.com/docs/identity-security/session-summaries/';

const ACTION = <button type="button">Unlock Session Summaries</button>;

function renderPromo(needsUpgrade?: boolean) {
  return render(
    <MemoryRouter>
      <SessionSummariesPromo
        primaryAction={ACTION}
        needsUpgrade={needsUpgrade}
      />
    </MemoryRouter>
  );
}

async function findCapabilityRow(label: string) {
  return (await screen.findByText(label)).closest('li');
}

beforeEach(() => {
  localStorage.clear();
});

// Both variants share the same title, so the intro is what tells them apart.
describe('choosing the copy', () => {
  it('offers the feature by default', () => {
    renderPromo();

    expect(
      screen.getByText(/^Teleport can process sessions like this/)
    ).toBeVisible();
  });

  it('asks to upgrade when flagged', () => {
    renderPromo(true);

    expect(
      screen.getByText(/^Teleport Identity Security can summarize/)
    ).toBeVisible();
  });
});

describe('the promotion panel', () => {
  it('renders whatever ask the caller supplies', () => {
    renderPromo();

    expect(screen.getByTestId('session-summaries-promo')).toBeVisible();
    expect(
      screen.getByRole('button', { name: 'Unlock Session Summaries' })
    ).toBeVisible();
  });

  it('offers the docs alongside the ask', () => {
    renderPromo();

    expect(screen.getByRole('link', { name: 'Learn more' })).toHaveAttribute(
      'href',
      DOCS_URL
    );
  });
});

describe('collapsing the panel', () => {
  function getDisclosure(name = 'Session Summaries') {
    return screen.getByRole('button', { name });
  }

  function getBody() {
    return document.getElementById(
      getDisclosure().getAttribute('aria-controls')
    );
  }

  it('starts expanded', () => {
    renderPromo();

    expect(getDisclosure()).toHaveAttribute('aria-expanded', 'true');
    expect(getBody()).not.toHaveAttribute('inert');
  });

  it('collapses from the title and keeps focus there', async () => {
    const user = userEvent.setup();

    renderPromo();

    await user.click(getDisclosure());

    expect(getDisclosure()).toHaveAttribute('aria-expanded', 'false');
    expect(getDisclosure()).toHaveFocus();
    expect(getBody()).toHaveAttribute('inert');
  });

  it('expands again from the title', async () => {
    const user = userEvent.setup();

    renderPromo();

    await user.click(getDisclosure());
    await user.click(getDisclosure());

    expect(getDisclosure()).toHaveAttribute('aria-expanded', 'true');
    expect(getBody()).not.toHaveAttribute('inert');
  });

  it('stays collapsed across visits', async () => {
    const user = userEvent.setup();

    const { unmount } = renderPromo();

    await user.click(getDisclosure());

    unmount();
    renderPromo();

    expect(getDisclosure()).toHaveAttribute('aria-expanded', 'false');
  });
});

describe('capability popovers', () => {
  it('opens a capability popover on keyboard focus', async () => {
    renderPromo();

    const row = await findCapabilityRow('Review the risk');

    expect(row).toHaveAttribute('tabindex', '0');

    act(() => row.focus());

    expect(
      await screen.findByText(/assigns a risk score to each session/)
    ).toBeInTheDocument();
  });

  it('closes the popover on Escape', async () => {
    const user = userEvent.setup();

    renderPromo();

    const row = await findCapabilityRow('Review the risk');

    act(() => row.focus());

    await screen.findByText(/assigns a risk score to each session/);

    await user.keyboard('{Escape}');

    await waitFor(() =>
      expect(
        screen.queryByText(/assigns a risk score to each session/)
      ).not.toBeInTheDocument()
    );
  });
});
