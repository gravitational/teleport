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

import { Platform, getPlatform } from 'design/platform';

import { TdpClient, ButtonState } from 'teleport/lib/tdp';
import { SyncKeys } from 'teleport/lib/tdp/codec';

/**
 * Handles keyboard events.
 */
export class KeyboardHandler {
  /**
   * Cache for keys whose down event has been withheld until the next keydown or keyup,
   * or to never be sent if this cache is cleared onfocusout.
   */
  private withheldDown: Map<string, boolean> = new Map();
  /**
   * Cache for keys whose up event has been delayed for 10ms.
   */
  private delayedUp: Map<string, NodeJS.Timeout> = new Map();
  /**
   * Tracks whether the next keydown or keyup event should sync the
   * local toggle key state to the remote machine.
   *
   * Set to true:
   * - On component initialization, so keys are synced before the first keydown/keyup event.
   * - On focusout, so keys are synced when the user returns to the window.
   */
  private syncBeforeNextKey: boolean = true;
  private isMac: boolean = getPlatform() === Platform.macOS;

  /**
   * Primary method for handling keyboard events.
   */
  public handleKeyboardEvent(
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ) {
    e.preventDefault();
    this.handleSyncBeforeNextKey(cli, e);
    // If this is a withheld or delayed key, handle it immediately and return.
    if (this.handleWithholdingAndDelay(cli, e, state)) {
      return;
    }

    this.finishHandlingKeyboardEvent(cli, e, state);
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
   * Called before every keydown or keyup event. This witholds the
   * keys for which we want to see what the next event is before
   * sending them on to the server, and sets up delayed up events.
   *
   * Returns true if the event was handled, false otherwise.
   */
  private handleWithholdingAndDelay(
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ): boolean {
    if (this.isWitholdableOrDelayeable(e) && state === ButtonState.DOWN) {
      // Unlikely, but theoretically possible. In order to ensure correctness,
      // we clear any delayed up event for this key and handle it immediately.
      const timeout = this.delayedUp.get(e.code);
      if (timeout) {
        clearTimeout(timeout);
        this.finishHandlingKeyboardEvent(cli, e, ButtonState.UP);
      }

      // Then we set the key down to be withheld until the next keydown or keyup,
      // or to never be sent if this cache is cleared onfocusout.
      this.withheldDown.set(e.code, true);

      return true;
    } else if (this.isWitholdableOrDelayeable(e) && state === ButtonState.UP) {
      // If we receive a delayed up event for a key that was already delayed,
      // we log a warning. This should never happen, because we can only get an
      // up event after a down, and we ensure the up cache is cleared when we
      // handle a down event.
      if (this.delayedUp.has(e.code)) {
        // eslint-disable-next-line no-console
        console.warn(
          'Received a delayed up event for a key that was already delayed. This should not happen.'
        );
      }

      const timeout = setTimeout(() => {
        this.finishHandlingKeyboardEvent(cli, e, ButtonState.UP);
      }, 10 /* ms */);

      // And add the timeout to the cache.
      this.delayedUp.set(e.code, timeout);

      return true;
    }

    return false;
  }

  private isWitholdableOrDelayeable(e: KeyboardEvent): boolean {
    return e.key === 'Meta' || e.key === 'Alt';
  }

  /**
   * Called to finish handling a keyboard event.
   *
   * For normal keys, this is called immediately.
   * For withheld or delayed keys, this is called as the callback when
   * another key is pressed or released (withheld) or after a delay (delayed).
   */
  private finishHandlingKeyboardEvent(
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ): void {
    // Release any withheld keys before sending the current key.
    this.sendWithheldKeys(cli);

    // Special handling for CapsLock on Mac.
    if (e.code === 'CapsLock' && this.isMac) {
      // On Mac, every UP or DOWN given to us by the browser corresponds
      // to a DOWN + UP on the remote machine for CapsLock.
      cli.sendKeyboardInput('CapsLock', ButtonState.DOWN);
      cli.sendKeyboardInput('CapsLock', ButtonState.UP);
    } else {
      // Otherwise, just pass the event through normally to the server.
      cli.sendKeyboardInput(e.code, state);
    }

    // If we're finishing handling for a withheld or delayed key, ensure
    // that it's cleared from the cache.
    this.clearWithholdingAndDelay(e.code);
  }

  /**
   * This sends all currently withheld keys to the server.
   */
  private sendWithheldKeys(cli: TdpClient) {
    this.withheldDown.forEach((value, code) => {
      if (value) {
        cli.sendKeyboardInput(code, ButtonState.DOWN);
      }
    });

    this.clearAllWithheldDown();
  }

  /**
   * Clears the withheld down cache for all keys.
   */
  private clearAllWithheldDown() {
    this.withheldDown.clear();
  }

  /**
   * Clears both caches for a single key.
   *
   * Calling this on a key that is not in the cache is a no-op.
   *
   * @param code The key code to clear the cache for.
   */
  private clearWithholdingAndDelay(code: string) {
    this.clearWithheldDown(code);
    this.clearDelayedUp(code);
  }

  /**
   * Clears the withheld down cache for a single key.
   *
   * Calling this on a key that is not in the cache is a no-op.
   *
   * @param code The key code to clear the cache for.
   */
  private clearWithheldDown(code: string) {
    this.withheldDown.delete(code);
  }

  /**
   * Clears the delayed up cache for a single key.
   *
   * Calling this on a key that is not in the cache is a no-op.
   *
   * @param code The key code to clear the cache for.
   */
  private clearDelayedUp(code: string) {
    const timeout = this.delayedUp.get(code);
    if (timeout !== undefined) {
      clearTimeout(timeout);
      this.delayedUp.delete(code);
    }
  }

  /**
   * Called when the canvas loses focus.
   *
   * This clears the withheld and delayed keys, so that they are not sent
   * to the server when the canvas is out of focus.
   */
  public onFocusOut() {
    this.clearAllWithholdingAndDelay();
    this.syncBeforeNextKey = true;
  }

  /**
   * To be called before unmounting the component.
   */
  public onUnmount() {
    this.clearAllDelayedUp();
  }

  private clearAllWithholdingAndDelay() {
    this.clearAllWithheldDown();
    this.clearAllDelayedUp();
  }

  /**
   * Clears the delayed up cache for all keys.
   */
  private clearAllDelayedUp() {
    this.delayedUp.forEach(timeout => {
      clearTimeout(timeout);
    });
    this.delayedUp.clear();
  }
}
