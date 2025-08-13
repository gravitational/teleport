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
import { ButtonState, MouseButton, SyncKeys, TdpClient } from 'shared/libs/tdp';

import { Withholder } from './Withholder';

/**
 * Handles mouse and keyboard events.
 */
export class InputHandler {
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

  /**
   * Keep track of the remote state of modifier keys.
   * This is used to mitigate stuck key issues when local
   * and remote modifier states are out of sync.
   */
  private remoteModifierState = new Map<string, ButtonState>([
    ['Shift', ButtonState.UP],
    ['Control', ButtonState.UP],
    ['Alt', ButtonState.UP],
    ['Meta', ButtonState.UP],
  ]);

  constructor() {
    // Bind finishHandlingInputEvent to this instance so it can be passed
    // as a callback to the Withholder.
    this.finishHandlingInputEvent = this.finishHandlingInputEvent.bind(this);
  }

  /**
   * Primary method for handling input events.
   */
  public handleInputEvent(params: InputEventParams) {
    const { e, cli, state } = params;
    if (e instanceof KeyboardEvent) {
      // Only prevent default for KeyboardEvents.
      // If preventDefault is done on MouseEvents,
      // it breaks focus and keys won't be registered.
      e.preventDefault();
      this.handleSyncBeforeNextKey(cli, e);
      this.updateModifierState(e.code, state);
    }
    this.withholder.handleInputEvent(params, this.finishHandlingInputEvent);
  }

  private handleSyncBeforeNextKey(
    cli: TdpClient,
    e: KeyboardEvent | MouseEvent
  ) {
    if (this.syncBeforeNextKey === true) {
      cli.sendSyncKeys(this.getSyncKeys(e));
      this.syncBeforeNextKey = false;
    }
  }

  private getSyncKeys = (e: KeyboardEvent | MouseEvent): SyncKeys => {
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
    e: KeyboardEvent | MouseEvent,
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
  private finishHandlingInputEvent(params: InputEventParams): void {
    const { cli, e, state } = params;

    // Synchronize local and remote modifier state before sending the input event.
    this.synchronizeModifierState(cli, e);

    // If this is a mouse event no special handling is needed.
    if (e instanceof MouseEvent) {
      cli.sendMouseButton(e.button as MouseButton, state);
      return;
    }

    // Special handling for CapsLock on Mac.
    if (e.code === 'CapsLock' && InputHandler.isMac) {
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
   * Updates the locally tracked remote modifier state.
   *
   * @param keyCode The key code of the key.
   * @param state The state of the key.
   */
  private updateModifierState(keyCode: string, state: ButtonState) {
    switch (keyCode) {
      case 'ShiftLeft':
      case 'ShiftRight':
        this.remoteModifierState.set('Shift', state);
        break;
      case 'ControlLeft':
      case 'ControlRight':
        this.remoteModifierState.set('Control', state);
        break;
      case 'AltLeft':
      case 'AltRight':
        this.remoteModifierState.set('Alt', state);
        break;
      case 'MetaLeft':
      case 'MetaRight':
        this.remoteModifierState.set('Meta', state);
        break;
    }
  }

  /**
   * Checks if the given key is a modifier key.
   *
   * @param key The key to check.
   * @returns True if the key is a modifier key, false otherwise.
   */
  private isModifierKey(key: string): boolean {
    return ['Shift', 'Control', 'Alt', 'Meta'].some(modifier =>
      key.startsWith(modifier)
    );
  }

  /**
   * Synchronizes the local modifier state with the remote machine.
   * This is called when key or mouse click/scroll events occur to ensure
   * that the remote machine has the correct state of modifier keys.
   * If not, it sends the current state.
   *
   * @param cli The TdpClient instance used to send the state.
   * @param e The KeyboardEvent or MouseEvent that triggered the synchronization.
   */
  public synchronizeModifierState(
    cli: TdpClient,
    e: KeyboardEvent | MouseEvent
  ) {
    // Don't process modifier keys themselves
    if (e instanceof KeyboardEvent && this.isModifierKey(e.code)) {
      return;
    }

    this.remoteModifierState.forEach((state, modifier) => {
      const localState = e.getModifierState(modifier)
        ? ButtonState.DOWN
        : ButtonState.UP;

      if (localState !== state) {
        // If the local state is different from the remote state, send the updates.
        cli.sendKeyboardInput(modifier + 'Left', localState);
        cli.sendKeyboardInput(modifier + 'Right', localState);
        // Update the remote state to match the local state.
        this.remoteModifierState.set(modifier, localState);
      }
    });
  }

  /**
   * Must be called when the element associated with the InputHandler loses focus.
   */
  public onFocusOut() {
    // Sync toggle keys when we come back into focus.
    this.syncBeforeNextKey = true;
    // Cancel any withheld keys.
    this.withholder.cancel();
  }

  /**
   * Should be called when the element associated with the InputHandler goes away.
   */
  public dispose() {
    // Make sure we cancel any withheld keys, particularly we want to cancel the timeouts.
    this.withholder.cancel();
  }
}

export type InputEventParams = {
  cli: TdpClient;
  e: KeyboardEvent | MouseEvent;
  state: ButtonState;
};
