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

import {
  ButtonState,
  selectDirectoryInBrowser,
  TdpClient,
} from 'shared/libs/tdp';

import { StuckKeys } from './StuckKeys';

jest.mock('teleport/lib/tdp', () => {
  const originalModule = jest.requireActual('shared/libs/tdp');
  return {
    ...originalModule,
    TdpClient: jest.fn().mockImplementation(() => {
      return {
        connect: jest.fn().mockResolvedValue(undefined),
        sendKeyboardInput: jest.fn(),
      };
    }),
  };
});

describe('StuckKeys', () => {
  jest.useFakeTimers();
  let stuckKeys: StuckKeys;
  let mockTdpClient: TdpClient;
  let mockSendKeyboardInput: jest.Mock;

  beforeEach(() => {
    stuckKeys = new StuckKeys();
    mockTdpClient = new TdpClient(() => null, selectDirectoryInBrowser);
    mockSendKeyboardInput = jest.fn();
    (mockTdpClient.sendKeyboardInput as jest.Mock) = mockSendKeyboardInput;
  });

  afterEach(() => {
    stuckKeys.cancel();
    mockSendKeyboardInput.mockClear();
    jest.clearAllTimers();
  });

  it('releases Meta and Shift keys after timeout when both are pressed', () => {
    stuckKeys.handleKeyboardEvent({
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    stuckKeys.handleKeyboardEvent({
      e: { key: 'Shift' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    expect(mockSendKeyboardInput).not.toHaveBeenCalled();

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS);

    expect(mockSendKeyboardInput).toHaveBeenCalledTimes(4);

    const callArgs = mockSendKeyboardInput.mock.calls.map(call => call);
    expect(callArgs).toEqual(
      expect.arrayContaining([
        ['MetaLeft', ButtonState.UP],
        ['MetaRight', ButtonState.UP],
        ['ShiftLeft', ButtonState.UP],
        ['ShiftRight', ButtonState.UP],
      ])
    );
  });

  it('does not release keys if one is released before the timeout', () => {
    stuckKeys.handleKeyboardEvent({
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    stuckKeys.handleKeyboardEvent({
      e: { key: 'Shift' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS / 2);

    stuckKeys.handleKeyboardEvent({
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: mockTdpClient,
    });

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS / 2);

    expect(mockSendKeyboardInput).not.toHaveBeenCalled();
  });

  it('resets the timeout when keys are pressed again', () => {
    stuckKeys.handleKeyboardEvent({
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    stuckKeys.handleKeyboardEvent({
      e: { key: 'Shift' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS / 2);

    stuckKeys.handleKeyboardEvent({
      e: { key: 'Shift' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: mockTdpClient,
    });

    stuckKeys.handleKeyboardEvent({
      e: { key: 'Shift' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS / 2);

    expect(mockSendKeyboardInput).not.toHaveBeenCalled();

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS);

    expect(mockSendKeyboardInput).toHaveBeenCalledTimes(4);
  });

  it('ignores unmonitored keys', () => {
    stuckKeys.handleKeyboardEvent({
      e: { key: 'A' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS);

    expect(mockSendKeyboardInput).not.toHaveBeenCalled();
  });

  it('cancels all timeouts and resets states when cancel is called', () => {
    stuckKeys.handleKeyboardEvent({
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    stuckKeys.handleKeyboardEvent({
      e: { key: 'Shift' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    stuckKeys.cancel();

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS);

    expect(mockSendKeyboardInput).not.toHaveBeenCalled();
  });

  it('does not release shift when used with regular keys after timeout', () => {
    stuckKeys.handleKeyboardEvent({
      e: { key: 'Shift' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    stuckKeys.handleKeyboardEvent({
      e: { key: 'A' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: mockTdpClient,
    });

    jest.advanceTimersByTime(stuckKeys.RELEASE_DELAY_MS * 2);

    expect(mockSendKeyboardInput).not.toHaveBeenCalled();
  });
});
