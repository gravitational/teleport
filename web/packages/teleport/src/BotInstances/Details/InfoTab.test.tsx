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

import { ComponentProps } from 'react';

import { screen, within } from 'design/utils/testing';

import cfg from 'teleport/config';
import { mockGetBotInstanceResponse } from 'teleport/test/helpers/botInstances';
import { renderWithMemoryRouter } from 'teleport/test/helpers/router';

import { InfoTab } from './InfoTab';

afterEach(() => {
  jest.clearAllMocks();
});

describe('InfoTab', () => {
  it('renders summary section', async () => {
    renderComponent();

    const section = screen
      .getByRole('heading', { name: 'Summary' })
      .closest('section');
    expect(section).toBeInTheDocument();

    expectFieldAndValue('Bot name', 'test-bot-name', section);
    expectFieldAndValue('Up time', '12h 1m', section);
    expectFieldAndValue('Kind', 'tctl', section);
    expectFieldAndValue('Version', 'v18.4.0', section);
    expectFieldAndValue('OS', 'linux', section);
    expectFieldAndValue('Hostname', 'test-hostname', section);
  });

  it('renders health section', async () => {
    renderComponent();

    const section = screen
      .getByRole('heading', { name: 'Health Status' })
      .closest('section');

    expect(
      within(section!).getByText(
        (_, element) => element?.textContent === '1 of 4 services are healthy'
      )
    ).toBeInTheDocument();

    expect(
      within(section!).getByText('application-tunnel-1', {})
    ).toBeInTheDocument();
    expect(within(section!).getByText('db-eu-lon-1', {})).toBeInTheDocument();
    expect(
      within(section!).getByText('workload-identity-aws-roles-anywhere-1', {})
    ).toBeInTheDocument();
    expect(
      within(section!).getByText('application-tunnel-2', {})
    ).toBeInTheDocument();
  });

  it('renders join token section', async () => {
    renderComponent();

    const section = screen
      .getByRole('heading', { name: 'Join Token' })
      .closest('section');
    expect(section).toBeInTheDocument();

    expectFieldAndValue('Name', 'test-token-name', section);
    expectFieldAndValue('Method', 'github', section);
    expectFieldAndValue('Repository', 'gravitational/teleport', section);
    expectFieldAndValue('Subject', 'test-github-sub', section);
  });

  it('navigate on bot name link click', async () => {
    const { user, router } = renderComponent();

    const section = screen
      .getByRole('heading', { name: 'Summary' })
      .closest('section');
    expect(section).toBeInTheDocument();

    const link = within(section!).getByText('test-bot-name');
    await user.click(link);

    expect(router.state.location.pathname).toBe('/web/bot/test-bot-name');
  });

  it('navigate on join token name link click', async () => {
    const { user, router } = renderComponent();

    const section = screen
      .getByRole('heading', { name: 'Join Token' })
      .closest('section');
    expect(section).toBeInTheDocument();

    const link = within(section!).getByText('test-token-name');
    await user.click(link);

    expect(router.state.location.pathname).toBe('/web/tokens');
  });

  it('callback on "view services" click', async () => {
    const callback = jest.fn();
    const { user } = renderComponent({ onGoToServicesClick: callback });

    const section = screen
      .getByRole('heading', { name: 'Health Status' })
      .closest('section');

    const button = within(section!).getByText('View Services');
    await user.click(button);

    expect(callback).toHaveBeenCalledTimes(1);
  });
});

function expectFieldAndValue(
  field: string,
  value: string,
  container?: HTMLElement | null
) {
  if (container) {
    expect(within(container).getByText(field)).toBeInTheDocument();
    expect(within(container).getByText(value)).toBeInTheDocument();
  } else {
    expect(screen.getByText(field)).toBeInTheDocument();
    expect(screen.getByText(value)).toBeInTheDocument();
  }
}

function renderComponent(props?: Partial<ComponentProps<typeof InfoTab>>) {
  const { data = mockGetBotInstanceResponse, onGoToServicesClick = jest.fn() } =
    props ?? {};

  return {
    ...renderWithMemoryRouter(
      <InfoTab data={data} onGoToServicesClick={onGoToServicesClick} />,
      {
        initialEntries: [cfg.getBotInstancesRoute()],
      }
    ),
  };
}
