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

import { QueryClientProvider } from '@tanstack/react-query';
import { setupServer } from 'msw/node';
import { ComponentProps, PropsWithChildren } from 'react';

import darkTheme from 'design/theme/themes/darkTheme';
import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitForElementToBeRemoved,
} from 'design/utils/testing';

import 'shared/components/TextEditor/TextEditor.mock';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  getBotInstanceError,
  getBotInstanceSuccess,
} from 'teleport/test/helpers/botInstances';

import { BotInstanceDetails } from './BotInstanceDetails';

const server = setupServer();

beforeAll(() => {
  server.listen();
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
});

afterAll(() => server.close());

describe('BotIntanceDetails', () => {
  it('Allows close action', async () => {
    const onClose = jest.fn();
    withSuccessResponse();

    const { user } = renderComponent({ onClose });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const closeButton = screen.getByLabelText('close');
    await user.click(closeButton);

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('Shows full yaml', async () => {
    withSuccessResponse();

    renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(
      screen.getByText('kind: bot_instance version: v1')
    ).toBeInTheDocument();
  });

  it('Shows an error', async () => {
    withErrorResponse();

    renderComponent();

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('something went wrong')).toBeInTheDocument();
  });

  it('Shows a permisison warning', async () => {
    withErrorResponse();

    renderComponent({
      hasBotInstanceReadPermission: false,
    });

    expect(
      screen.getByText('You do not have permission to read Bot instances', {
        exact: false,
      })
    ).toBeInTheDocument();

    expect(screen.getByText('bot_instance.read')).toBeInTheDocument();
  });
});

const renderComponent = (
  options?: Partial<ComponentProps<typeof BotInstanceDetails>> & {
    hasBotInstanceReadPermission?: boolean;
  }
) => {
  const {
    botName = 'test-bot-name',
    instanceId = '4fa10e68-f2e0-4cf9-ad5b-1458febcd827',
    onClose = jest.fn(),
    ...rest
  } = options ?? {};
  const user = userEvent.setup();
  return {
    ...render(
      <BotInstanceDetails
        botName={botName}
        instanceId={instanceId}
        onClose={onClose}
      />,
      {
        wrapper: makeWrapper(rest),
      }
    ),
    user,
  };
};

function makeWrapper(options?: { hasBotInstanceReadPermission?: boolean }) {
  const { hasBotInstanceReadPermission = true } = options ?? {};

  const customAcl = makeAcl({
    botInstances: {
      ...defaultAccess,
      read: hasBotInstanceReadPermission,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });
  return (props: PropsWithChildren) => {
    return (
      <QueryClientProvider client={testQueryClient}>
        <TeleportProviderBasic teleportCtx={ctx}>
          <ConfiguredThemeProvider theme={darkTheme}>
            {props.children}
          </ConfiguredThemeProvider>
        </TeleportProviderBasic>
      </QueryClientProvider>
    );
  };
}

const withSuccessResponse = () => {
  server.use(
    getBotInstanceSuccess({
      bot_instance: {
        spec: {
          instance_id: '4fa10e68-f2e0-4cf9-ad5b-1458febcd827',
        },
      },
      yaml: 'kind: bot_instance\nversion: v1\n',
    })
  );
};

const withErrorResponse = () => {
  server.use(getBotInstanceError(500, 'something went wrong'));
};
