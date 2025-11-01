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

import userEvent from '@testing-library/user-event';
import { ComponentProps, PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import { render, screen, within } from 'design/utils/testing';

import { mockGetBotInstanceResponse } from 'teleport/test/helpers/botInstances';

import { HealthTab } from './HealthTab';

beforeAll(() => {
  jest.useFakeTimers({
    now: new Date('2025-10-10T11:00:00Z'),
  });
});

afterAll(() => {
  jest.useRealTimers();
});

describe('HealthTab', () => {
  // eslint-disable-next-line jest/expect-expect
  it('renders', async () => {
    renderComponent();

    expectItem({
      name: 'application-tunnel-1',
      type: 'application-tunnel',
      updatedAt: 'Reported 15 minutes ago',
      status: 'Healthy',
    });

    expectItem({
      name: 'db-eu-lon-1',
      type: 'database-tunnel',
      updatedAt: 'Reported 14 minutes ago',
      status: 'Unhealthy',
      reason:
        'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
    });

    expectItem({
      name: 'workload-identity-aws-roles-anywhere-1',
      type: 'workload-identity-aws-roles-anywhere',
      updatedAt: 'Reported 13 minutes ago',
      status: 'Initializing',
    });

    expectItem({
      name: 'application-tunnel-2',
      type: 'application-tunnel',
      updatedAt: 'Reported 12 minutes ago',
      status: 'Unknown',
    });
  });

  it('show an empty state', async () => {
    renderComponent({
      data: {
        bot_instance: {
          status: {
            service_health: [],
          },
        },
      },
    });

    expect(screen.getByText('No reported services')).toBeInTheDocument();
  });
});

function expectItem(match: {
  name: string;
  type: string;
  updatedAt: string;
  status: string;
  reason?: string;
}) {
  const item = screen.getByTestId(match.name);
  expect(within(item).getByText(match.name)).toBeInTheDocument();
  expect(within(item).getByText(`Type: ${match.type}`)).toBeInTheDocument();
  expect(within(item).getByText(match.updatedAt)).toBeInTheDocument();
  expect(within(item).getByText(match.status)).toBeInTheDocument();
  if (match.reason) {
    expect(within(item).getByText(match.reason)).toBeInTheDocument();
  }
}

function renderComponent(props?: Partial<ComponentProps<typeof HealthTab>>) {
  const { data = mockGetBotInstanceResponse } = props ?? {};
  const user = userEvent.setup();

  return {
    ...render(<HealthTab data={data} />, { wrapper: makeWrapper() }),
    user,
    history,
  };
}

function makeWrapper() {
  return (props: PropsWithChildren) => (
    <ConfiguredThemeProvider theme={darkTheme}>
      {props.children}
    </ConfiguredThemeProvider>
  );
}
