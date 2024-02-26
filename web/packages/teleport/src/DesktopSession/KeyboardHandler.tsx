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
  private isMac: boolean = getPlatform() === Platform.macOS;

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
    if (e.code === 'CapsLock' && this.isMac) {
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

/**
 * The Withholder class manages keyboard events, particularly for alt/cmd keys. It delays handling these keys to determine the user's intent:
 *
 * - For alt/cmd DOWN events, it waits to see if this is part of a normal operation or an alt/cmd + tab action. In the latter case, it cancels
 * the event to prevent the remote server from mistakenly thinking the key is held down when the user returns to the browser window, which can
 * cause issues as described in https://github.com/gravitational/teleport/issues/24342.
 * - For alt/cmd UP events, it introduces a short delay before handling to avoid unintended actions (like opening the start menu), in the case
 * that an alt/cmd + tab registers both the alt/cmd DOWN and UP events in quick succession. (This can happen if the user does an alt/cmd + tab
 * really quickly, according to our testing.)
 * - For other keys, it handles them immediately.
 *
 * Events are either processed immediately, delayed, or cancelled based on user actions and focus changes.
 */
export class Withholder {
  /**
   * The list of keys which are to be withheld.
   */
  private keysToWithhold: string[] = ['Meta', 'Alt'];
  /**
   * The internal map of keystrokes that are currently
   * being withheld.
   */
  private withheldKeys: Array<WithheldKeyboardEventHandler> = [];

  /**
   * All keyboard events should be handled via this function.
   */
  public handleKeyboardEvent(
    params: KeyboardEventParams,
    handleKeyboardEvent: (params: KeyboardEventParams) => void
  ) {
    const key = params.e.key;

    // If this is not a key we withhold, immediately flush any withheld keys
    // and handle this key.
    if (!this.keysToWithhold.includes(key)) {
      this.flush();
      handleKeyboardEvent(params);
      return;
    }

    // This is a key we withhold:

    // On key down we withhold without a timeout. The handler will ultimately be called
    // when the key is released (typically after a timeout, see the comment in the
    // conditional below), or when another key is pressed, or else it should be cancelled
    // onfocusout or on unmount.
    let timeout = undefined;
    if (params.state === ButtonState.UP) {
      // On key ups we withhold on a timeout. The function will be called when the
      // timer times out, or when another key is pressed, or else it should be
      // cancelled onfocusout or on unmount.
      timeout = setTimeout(() => {
        // Just flush after the timeout, the handler will
        // be in the queue by then and thus will be called.
        //
        // Technically this might flush some keys that were
        // pressed after this one, but that works okay in practice.
        // A user would have to be doing something extremely unusual
        // for this to become a noticeable problem.
        this.flush();
      }, 10); // 10 ms was determined empirically to work well.
    }

    this.withheldKeys.push({
      params,
      handler: handleKeyboardEvent,
      timeout,
    });
  }

  // Cancel all withheld keys.
  public cancel() {
    while (this.withheldKeys.length > 0) {
      const withheld = this.withheldKeys.shift();
      if (withheld && withheld.timeout) {
        clearTimeout(withheld.timeout);
      }
    }
  }

  // Flush all withheld keys.
  private flush() {
    while (this.withheldKeys.length > 0) {
      const withheld = this.withheldKeys.shift();
      if (withheld) {
        const { handler, params, timeout } = withheld;
        if (timeout) {
          clearTimeout(timeout);
        }
        handler(params);
      }
    }
  }
}

type WithheldKeyboardEventHandler = {
  handler: (params: KeyboardEventParams) => void;
  params: KeyboardEventParams;
  timeout?: NodeJS.Timeout;
};

type KeyboardEventParams = {
  cli: TdpClient;
  e: KeyboardEvent;
  state: ButtonState;
};
