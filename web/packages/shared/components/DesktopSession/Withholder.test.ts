/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { BrowserFileSystem, ButtonState, TdpClient } from 'shared/libs/tdp';

import { Withholder } from './Withholder';

// Mock the TdpClient class
jest.mock('teleport/lib/tdp', () => {
  const originalModule = jest.requireActual('shared/libs/tdp'); // Get the actual module
  return {
    ...originalModule,
    TdpClient: jest.fn().mockImplementation(() => {
      return {
        connect: jest.fn().mockResolvedValue(undefined),
      };
    }),
  };
});

describe('withholder', () => {
  jest.useFakeTimers();
  let withholder: Withholder;
  let mockHandleKeyboardEvent: jest.Mock;

  beforeEach(() => {
    withholder = new Withholder();
    mockHandleKeyboardEvent = jest.fn();
  });

  afterEach(() => {
    withholder.cancel();
    mockHandleKeyboardEvent.mockClear();
    jest.clearAllTimers();
  });

  it('handles non-withheld keys immediately', () => {
    const params = {
      e: { key: 'Enter' } as KeyboardEvent as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: new TdpClient(() => null, new BrowserFileSystem()),
    };
    withholder.handleKeyboardEvent(params, mockHandleKeyboardEvent);
    expect(mockHandleKeyboardEvent).toHaveBeenCalledWith(params);
  });

  it('flushes withheld keys upon non-withheld key press', () => {
    const metaDown = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: new TdpClient(() => null, new BrowserFileSystem()),
    };

    const metaUp = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient(() => null, new BrowserFileSystem()),
    };

    const enterDown = {
      e: { key: 'Enter' } as KeyboardEvent as KeyboardEvent,
      state: ButtonState.DOWN,
      cli: new TdpClient(() => null, new BrowserFileSystem()),
    };

    withholder.handleKeyboardEvent(metaDown, mockHandleKeyboardEvent);
    withholder.handleKeyboardEvent(metaUp, mockHandleKeyboardEvent);

    expect(mockHandleKeyboardEvent).not.toHaveBeenCalled();

    withholder.handleKeyboardEvent(enterDown, mockHandleKeyboardEvent);

    expect(mockHandleKeyboardEvent).toHaveBeenCalledTimes(3);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(1, metaDown);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(2, metaUp);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(3, enterDown);
  });

  it('withholds Meta/Alt UP and then handles them on a timer', () => {
    const metaParams = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient(() => null, new BrowserFileSystem()),
    };
    const altParams = {
      e: { key: 'Alt' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient(() => null, new BrowserFileSystem()),
    };

    withholder.handleKeyboardEvent(metaParams, mockHandleKeyboardEvent);
    withholder.handleKeyboardEvent(altParams, mockHandleKeyboardEvent);

    expect(mockHandleKeyboardEvent).not.toHaveBeenCalled();

    jest.advanceTimersByTime(10);

    expect(mockHandleKeyboardEvent).toHaveBeenCalledTimes(2);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(1, metaParams);
    expect(mockHandleKeyboardEvent).toHaveBeenNthCalledWith(2, altParams);
  });

  it('cancels withheld keys correctly', () => {
    const metaParams = {
      e: { key: 'Meta' } as KeyboardEvent,
      state: ButtonState.UP,
      cli: new TdpClient(() => null, new BrowserFileSystem()),
    };
    withholder.handleKeyboardEvent(metaParams, mockHandleKeyboardEvent);
    expect((withholder as any).withheldKeys).toHaveLength(1);

    withholder.cancel();
    jest.advanceTimersByTime(10);

    expect(mockHandleKeyboardEvent).not.toHaveBeenCalled();
    expect((withholder as any).withheldKeys).toHaveLength(0);
  });
});
