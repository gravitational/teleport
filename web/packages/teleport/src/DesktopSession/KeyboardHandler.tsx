/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { getPlatform, Platform } from 'design/platform';

import { ButtonState, TdpClient } from 'teleport/lib/tdp';
import { SyncKeys } from 'teleport/lib/tdp/codec';

import { Withholder } from './Withholder';

/**
 * Handles keyboard events.
 */
export class KeyboardHandler {
  private withholder: Withholder = new Withholder();
  /**
   * Tracks whether the next keydown or keyup event should sync the
   * local toggle key state to the remote machine.
   *
   * Set to true:
   * - On component initialization, so keys are synced before the first keydown/keyup event.
   * - On focusout, so keys are synced when the user returns to the window.
   */
  private syncBeforeNextKey: boolean = true;
  private static isMac: boolean = getPlatform() === Platform.macOS;

  constructor() {
    // Bind finishHandlingKeyboardEvent to this instance so it can be passed
    // as a callback to the Withholder.
    this.finishHandlingKeyboardEvent =
      this.finishHandlingKeyboardEvent.bind(this);
  }

  /**
   * Primary method for handling keyboard events.
   */
  public handleKeyboardEvent(params: KeyboardEventParams) {
    const { e, cli } = params;
    e.preventDefault();
    this.handleSyncBeforeNextKey(cli, e);
    this.withholder.handleKeyboardEvent(
      params,
      this.finishHandlingKeyboardEvent
    );
  }

  private handleSyncBeforeNextKey(cli: TdpClient, e: KeyboardEvent) {
    if (this.syncBeforeNextKey === true) {
      cli.sendSyncKeys(this.getSyncKeys(e));
      this.syncBeforeNextKey = false;
    }
  }

  private getSyncKeys = (e: KeyboardEvent): SyncKeys => {
    return {
      scrollLockState: this.getModifierState(e, 'ScrollLock'),
      numLockState: this.getModifierState(e, 'NumLock'),
      capsLockState: this.getModifierState(e, 'CapsLock'),
      kanaLockState: ButtonState.UP, // KanaLock is not supported, see https://www.w3.org/TR/uievents-key/#keys-modifier
    };
  };

  /**
   * Returns the ButtonState corresponding to the given `keyArg`.
   *
   * @param e The `KeyboardEvent`
   * @param keyArg The key to check the state of. Valid values can be found [here](https://www.w3.org/TR/uievents-key/#keys-modifier)
   */
  private getModifierState = (
    e: KeyboardEvent,
    keyArg: string
  ): ButtonState => {
    return e.getModifierState(keyArg) ? ButtonState.DOWN : ButtonState.UP;
  };

  /**
   * Called to finish handling a keyboard event.
   *
   * For normal keys, this is called immediately.
   * For withheld or delayed keys, this is called as the callback when
   * another key is pressed or released (withheld) or after a delay (delayed).
   */
  private finishHandlingKeyboardEvent(params: KeyboardEventParams): void {
    const { cli, e, state } = params;
    // Special handling for CapsLock on Mac.
    if (e.code === 'CapsLock' && KeyboardHandler.isMac) {
      // On Mac, every UP or DOWN given to us by the browser corresponds
      // to a DOWN + UP on the remote machine for CapsLock.
      cli.sendKeyboardInput('CapsLock', ButtonState.DOWN);
      cli.sendKeyboardInput('CapsLock', ButtonState.UP);
    } else {
      // Otherwise, just pass the event through normally to the server.
      cli.sendKeyboardInput(e.code, state);
    }
  }

  /**
   * Must be called when the element associated with the KeyboardHandler loses focus.
   */
  public onFocusOut() {
    // Sync toggle keys when we come back into focus.
    this.syncBeforeNextKey = true;
    // Cancel any withheld keys.
    this.withholder.cancel();
  }

  /**
   * Should be called when the element associated with the KeyboardHandler goes away.
   */
  public dispose() {
    // Make sure we cancel any withheld keys, particularly we want to cancel the timeouts.
    this.withholder.cancel();
  }
}

export type KeyboardEventParams = {
  cli: TdpClient;
  e: KeyboardEvent;
  state: ButtonState;
};
