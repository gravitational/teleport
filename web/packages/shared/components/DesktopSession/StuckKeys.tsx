/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { ButtonState, TdpClient } from 'shared/libs/tdp';

import { KeyboardEventParams } from './KeyboardHandler';

interface KeyState {
  down: boolean;
}

/**
 * Timeout period (ms) after which we assume keys are stuck and auto-release them
 */
export const RELEASE_DELAY_MS = 1000;

/**
 * KeyCombo represents a combination of keys that are pressed together to
 * trigger shortcuts. It tracks the keys, their states, and manages timeouts
 * for releasing them.
 */
class KeyCombo {
  /**
   * Set of keys that are part of this combo.
   */
  keys: Set<string>;

  /**
   * Timeout ID for releasing the keys after a delay.
   */
  private timeout?: number;

  /**
   * Reference to the parent StuckKeys' key states map to track down/up state.
   */
  private keyStates: Map<string, KeyState>;

  /**
   * Creates a new KeyCombo instance.
   *
   * @param keys - Set of key names to track as a combination e.g., new Set(['Meta', 'Shift'])
   * @param keyStates - Reference to the parent StuckKeys' key states map to track down/up state
   */
  constructor(keys: Set<string>, keyStates: Map<string, KeyState>) {
    this.keys = new Set(keys);
    this.timeout = undefined;
    this.keyStates = keyStates;
  }

  /**
   * Checks if all keys in the combo are currently pressed down.
   */
  isActive(): boolean {
    return Array.from(this.keys).every(key => this.keyStates.get(key)?.down);
  }

  /**
   * Releases the keys in the combo by sending keyup events to the TDP client
   * and updating global key state.
   * 
   * @param cli - The TDP client to send the keyup events to.
   */
  release(cli: TdpClient): void {
    // For reach key in the key combo
    this.keys.forEach(key => {
      // Get the key code for the key and send a keyup event
      this.getKeyCode(key).forEach(keyCode => {
        cli.sendKeyboardInput(keyCode, ButtonState.UP);
      });

      // Update the key state to indicate it's no longer down
      if (this.keyStates.has(key)) {
        this.keyStates.get(key)!.down = false;
      }
    });

    this.timeout = undefined;
  }

  /**
   * Cancels the combo by clearing the timeout.
   */
  cancel(): void {
    if (this.timeout) {
      window.clearTimeout(this.timeout);
      this.timeout = undefined;
    }
  }

  handleComboState(combo: KeyCombo, cli: TdpClient) {
    // Clear the timeout if it exists because key state has changed
    combo.cancel();

    // If the combo is active, set a timeout to release it
    if (combo.isActive()) {
      combo.timeout = window.setTimeout(() => {
        combo.release(cli);
      }, RELEASE_DELAY_MS);
    }
  }

  /**
   * Returns the key codes for a given key.
   *
   * @param key - The key to get the codes for.
   */
  private getKeyCode(key: string): string[] {
    switch (key) {
      case 'Meta':
        return ['MetaLeft', 'MetaRight'];
      case 'Shift':
        return ['ShiftLeft', 'ShiftRight'];
      default:
        return [key];
    }
  }
}

/**
 * StuckKeys tracks key states and automatically sends keyup events after a timeout
 * for potentially stuck keys. This prevents issues where the remote system thinks
 * modifier keys are still pressed when they're not.
 *
 * This addresses the issue of Meta and Shift keys getting "stuck" when users
 * take a screenshot with Meta+Shift+[3 | 4 | 5] on macOS. When macOS steals focus during
 * screenshot capture, the browser does not receive keyup events for these keys.
 * It also doesn't trigger blur, focusout, or visibilitychange events. This class
 * can be extended to handle other key combinations that may get stuck.
 */
export class StuckKeys {
  /**
   * Keys to monitor
   */
  private keyStates = new Map<string, KeyState>();

  /**
   * Key combinations to check for stuck state
   */
  private keyCombos: KeyCombo[] = [];

  constructor() {
    this.addKeyCombo(new Set(['Meta', 'Shift']));
  }
  
  /**
   * Process keyboard events and manage potential stuck keys.
   *
   * This function only sends key up events for keys that may
   * be stuck, it will not forward any other keys to the server,
   * so make sure they're handled by the KeyboardHandler.
   */
  public handleKeyboardEvent(params: KeyboardEventParams) {
    const { e, state, cli } = params;
    const key = e.key;

    // Exit early if the key is not one we need to monitor
    if (!this.keyStates.has(key)) {
      return;
    }

    // Update key state
    this.keyStates.get(key)!.down = state === ButtonState.DOWN;

    // Check all key combinations to see if any are active
    this.keyCombos.forEach(combo => {
      if (combo.keys.has(key)) {
        combo.handleComboState(combo, cli);
      }
    });
  }

  // Add cancel function to clear timeouts and reset key states
  public cancel() {
    this.keyCombos.forEach(combo => {
      combo.cancel();
    });
    this.keyStates.forEach(state => (state.down = false));
  }

  /**
   * Adds a new key combination to monitor.
   *
   * @param keys - Set of key names to track as a combination (e.g., ['Meta', 'Shift'])
   */
  private addKeyCombo(keys: Set<string>) {
    if (keys.size === 0) {
      return;
    }

    // Check if the combo already exists
    if (this.keyCombos.some(combo =>
      Array.from(combo.keys).every(key => keys.has(key))
    )) {
      return;
    }

    // Create the key combo and add it to the list
    const combo = new KeyCombo(keys, this.keyStates);
    this.keyCombos.push(combo);

    // Add these keys to the key states map
    keys.forEach(key => {
      if (!this.keyStates.has(key)) {
        this.keyStates.set(key, { down: false });
      }
    });
  }
}
