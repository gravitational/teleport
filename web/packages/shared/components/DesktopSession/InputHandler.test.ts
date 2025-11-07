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

import {
  ButtonState,
  selectDirectoryInBrowser,
  TdpClient,
} from 'shared/libs/tdp';

import { InputHandler } from './InputHandler';

// Mock the TdpClient class
vi.mock('shared/libs/tdp', async () => {
  const originalModule = await vi.importActual('shared/libs/tdp');
  return {
    ...originalModule,
    TdpClient: vi.fn().mockImplementation(() => {
      return {
        sendKeyboardInput: vi.fn(),
        sendMouseButton: vi.fn(),
        sendSyncKeys: vi.fn(),
      };
    }),
  };
});

describe('InputHandler', () => {
  let inputHandler: InputHandler;
  let mockTdpClient: TdpClient;

  beforeEach(() => {
    inputHandler = new InputHandler();
    mockTdpClient = new TdpClient(
      () => null,
      selectDirectoryInBrowser
    );
  });

  afterEach(() => {
    inputHandler.dispose();
  });

  describe('synchronizeModifierState', () => {
    it('sends modifier sync when local and remote states differ', () => {
      // Create event with Shift pressed but don't track it in remote state first
      const event = new KeyboardEvent('keydown', {
        code: 'KeyA',
        shiftKey: true,
      });

      const params = {
        e: event,
        state: ButtonState.DOWN,
        cli: mockTdpClient,
      };

      inputHandler.handleInputEvent(params);

      // Should send Shift DOWN to sync states since remote state defaults to UP
      expect(mockTdpClient.sendKeyboardInput).toHaveBeenCalledWith(
        'ShiftLeft',
        ButtonState.DOWN
      );
    });

    it('does not send sync when states are already synchronized', () => {
      // First, set Shift as DOWN in remote state
      const shiftDownEvent = new KeyboardEvent('keydown', {
        code: 'ShiftLeft',
      });
      inputHandler.handleInputEvent({
        e: shiftDownEvent,
        state: ButtonState.DOWN,
        cli: mockTdpClient,
      });

      mockTdpClient.sendKeyboardInput.mockClear();

      // Now press a key with Shift
      const event = new KeyboardEvent('keydown', {
        code: 'KeyA',
        shiftKey: true,
      });

      inputHandler.handleInputEvent({
        e: event,
        state: ButtonState.DOWN,
        cli: mockTdpClient,
      });

      // Should not send any Shift synchronization events
      const shiftCalls = mockTdpClient.sendKeyboardInput.mock.calls.filter(
        call => call[0].includes('Shift')
      );
      expect(shiftCalls).toHaveLength(0);
    });

    it('synchronizes multiple modifier states correctly', () => {
      // Set Alt as DOWN in remote state (to test it gets synced to UP)
      const altDownEvent = new KeyboardEvent('keydown', { code: 'AltLeft' });
      inputHandler.handleInputEvent({
        e: altDownEvent,
        state: ButtonState.DOWN,
        cli: mockTdpClient,
      });

      // Press event with multiple modifiers active but not previously tracked
      const event = new KeyboardEvent('keydown', {
        code: 'KeyA',
        shiftKey: true,
        ctrlKey: true,
        altKey: false,
        metaKey: false,
      });

      inputHandler.handleInputEvent({
        e: event,
        state: ButtonState.DOWN,
        cli: mockTdpClient,
      });

      // Should sync Shift and Control to DOWN, and Alt to UP
      expect(mockTdpClient.sendKeyboardInput).toHaveBeenCalledWith(
        'ShiftLeft',
        ButtonState.DOWN
      );
      expect(mockTdpClient.sendKeyboardInput).toHaveBeenCalledWith(
        'ControlLeft',
        ButtonState.DOWN
      );
      expect(mockTdpClient.sendKeyboardInput).toHaveBeenCalledWith(
        'AltLeft',
        ButtonState.UP
      );
    });

    it('handles modifier key release correctly', () => {
      // First press Shift
      const shiftDownEvent = new KeyboardEvent('keydown', {
        code: 'ShiftLeft',
      });
      inputHandler.handleInputEvent({
        e: shiftDownEvent,
        state: ButtonState.DOWN,
        cli: mockTdpClient,
      });

      // Then release Shift
      const shiftUpEvent = new KeyboardEvent('keyup', { code: 'ShiftLeft' });
      inputHandler.handleInputEvent({
        e: shiftUpEvent,
        state: ButtonState.UP,
        cli: mockTdpClient,
      });

      mockTdpClient.sendKeyboardInput.mockClear();

      // Now press a key without Shift
      const normalKeyEvent = new KeyboardEvent('keydown', {
        code: 'KeyA',
        shiftKey: false,
      });

      inputHandler.handleInputEvent({
        e: normalKeyEvent,
        state: ButtonState.DOWN,
        cli: mockTdpClient,
      });

      // Should not send additional Shift events since it's already UP
      const shiftCalls = mockTdpClient.sendKeyboardInput.mock.calls.filter(
        call => call[0].includes('Shift')
      );
      expect(shiftCalls).toHaveLength(0);
    });
  });
});
