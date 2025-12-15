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

import { setupServer } from 'msw/node';
import { PropsWithChildren } from 'react';

import {
  Providers,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { deleteBotError, deleteBotSuccess } from 'teleport/test/helpers/bots';

import { DeleteDialog } from './DeleteDialog';

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

describe('DeleteDialog', () => {
  it('should render correctly', async () => {
    renderComponent();
    expect(screen.getByText('Delete test-bot-name?')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Alternatively, you can lock a bot to stop all of its activity immediately.'
      )
    ).toBeInTheDocument();
    expect(screen.getByText('Delete Bot')).toBeEnabled();
    expect(screen.getByText('Cancel')).toBeEnabled();
    expect(screen.getByText('Lock Bot')).toBeEnabled();
  });

  it('should cancel', async () => {
    const onCancel = jest.fn();
    const { user } = renderComponent({
      onCancel,
    });
    await user.click(screen.getByText('Cancel'));
    expect(onCancel).toHaveBeenCalled();
  });

  it('should request lock', async () => {
    const onLockRequest = jest.fn();
    const { user } = renderComponent({
      onLockRequest,
    });
    await user.click(screen.getByText('Lock Bot'));
    expect(onLockRequest).toHaveBeenCalled();
  });

  it('should disable request lock', async () => {
    const onLockRequest = jest.fn();
    const { user } = renderComponent({
      onLockRequest,
      canLockBot: false,
    });
    await user.click(screen.getByText('Lock Bot'));
    expect(onLockRequest).not.toHaveBeenCalled();
    expect(screen.getByText('Lock Bot')).toBeDisabled();
  });

  it('should hide request lock', async () => {
    renderComponent({
      showLockAlternative: false,
    });
    expect(
      screen.queryByText(
        'Alternatively, you can lock a bot to stop all of its activity immediately.'
      )
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Lock Bot')).not.toBeInTheDocument();
  });

  it('should submit', async () => {
    withDeleteBotSuccess();
    const onComplete = jest.fn();
    const { user } = renderComponent({
      onComplete,
    });
    await user.click(screen.getByText('Delete Bot'));
    await waitFor(
      () => {
        expect(onComplete).toHaveBeenCalled();
      },
      { timeout: 5000 }
    );
  });

  it('should show submit error', async () => {
    withDeleteBotError();
    const onComplete = jest.fn();
    const { user } = renderComponent({
      onComplete,
    });
    await user.click(screen.getByText('Delete Bot'));
    await waitFor(() => {
      expect(screen.getByText('something went wrong')).toBeInTheDocument();
    });
    expect(onComplete).not.toHaveBeenCalled();
  });
});

function renderComponent(options?: {
  onCancel?: () => void;
  onComplete?: () => void;
  customAcl?: ReturnType<typeof makeAcl>;
  showLockAlternative?: boolean;
  canLockBot?: boolean;
  onLockRequest?: () => void;
}) {
  const {
    onCancel = jest.fn(),
    onComplete = jest.fn(),
    customAcl,
    showLockAlternative = true,
    canLockBot = true,
    onLockRequest = jest.fn(),
  } = options ?? {};
  const user = userEvent.setup();
  return {
    ...render(
      <DeleteDialog
        botName="test-bot-name"
        onCancel={onCancel}
        onComplete={onComplete}
        showLockAlternative={showLockAlternative}
        canLockBot={canLockBot}
        onLockRequest={onLockRequest}
      />,
      { wrapper: makeWrapper({ customAcl }) }
    ),
    user,
  };
}

function makeWrapper(params?: { customAcl?: ReturnType<typeof makeAcl> }) {
  const {
    customAcl = makeAcl({
      bots: {
        ...defaultAccess,
        read: true,
        edit: true,
      },
      roles: {
        ...defaultAccess,
        list: true,
      },
    }),
  } = params ?? {};
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });
    return (
      <Providers>
        <TeleportProviderBasic teleportCtx={ctx}>
          {children}
        </TeleportProviderBasic>
      </Providers>
    );
  };
}

function withDeleteBotSuccess() {
  server.use(deleteBotSuccess());
}

function withDeleteBotError() {
  server.use(deleteBotError(500, 'something went wrong'));
}
