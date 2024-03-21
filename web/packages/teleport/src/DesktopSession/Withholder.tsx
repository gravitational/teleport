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

import { ButtonState } from 'teleport/lib/tdp';

import { KeyboardEventParams } from './KeyboardHandler';

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
   * The internal array of keystrokes that are currently
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
      }, UP_DELAY_MS);
    }

    this.withheldKeys.push({
      params,
      handler: handleKeyboardEvent,
      timeout,
    });
  }

  // Cancel all withheld keys.
  public cancel() {
    this.withheldKeys.forEach(w => clearTimeout(w.timeout));
    this.withheldKeys = [];
  }

  // Flush all withheld keys.
  private flush() {
    this.withheldKeys.forEach(w => {
      clearTimeout(w.timeout);
      w.handler(w.params);
    });
    this.withheldKeys = [];
  }
}

type WithheldKeyboardEventHandler = {
  handler: (params: KeyboardEventParams) => void;
  params: KeyboardEventParams;
  timeout?: NodeJS.Timeout;
};

// 10 ms was determined empirically to work well.
const UP_DELAY_MS = 10;
