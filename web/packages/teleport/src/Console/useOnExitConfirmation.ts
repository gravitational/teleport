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

import { useEffect } from 'react';

import session from 'teleport/services/websession';

import ConsoleContext from './consoleContext';
import * as stores from './stores/types';

// TAB_MIN_AGE defines "active terminal" session in ms
const TAB_MIN_AGE = 30000;

/**
 * useOnExitConfirmation notifies users closing active terminal sessions by:
 *    refresh, window close, window tab close, session tab close.
 *
 * "active terminal" is defined by seconds the tab has been opened.
 *
 * @param ctx data that is shared between Console related components.
 */
function useOnExitConfirmation(ctx: ConsoleContext) {
  useEffect(() => {
    /**
     * handleBeforeUnload listens for browser closes and refreshes.
     * Checks if users need to be notified before closing based on type
     * of document opened and how long it has been active for.
     */
    const handleBeforeunload = event => {
      // Do not ask for confirmation when session is expired, which may trigger prompt
      // when browser triggers page reload before it receives session.end event,
      // which is not guaranteed to happen in that order.
      if (!session.isValid()) {
        return;
      }

      const shouldNotify = ctx.getDocuments().some(hasLastingSshConnection);

      if (shouldNotify) {
        // cancel event as set by standard, but not supported in all browsers
        event.preventDefault();
        // required in chrome
        event.returnValue = '';
      }
    };

    // add event listener on mount
    window.addEventListener('beforeunload', handleBeforeunload);

    return () => {
      window.removeEventListener('beforeunload', handleBeforeunload);
    };
  }, []);

  /**
   * hasLastingSshConnection calculates the milliseconds between given date
   * from when fn was called.
   *
   * @param doc the document in context
   */
  function hasLastingSshConnection(doc: stores.Document) {
    if (doc.kind !== 'terminal' || doc.status !== 'connected') {
      return false;
    }

    const created = doc.created.getTime();
    const fromNow = new Date().getTime();

    return fromNow - created > TAB_MIN_AGE;
  }

  /**
   * verifyAndConfirm verifies the document is of type terminal,
   * and based on how long it was active for, prompts users to confirm closing.
   *
   * A return value of true either means, user has confirmed or confirmation
   * is not required.
   *
   * @param doc the document in context
   */
  function verifyAndConfirm(doc: stores.Document) {
    if (hasLastingSshConnection(doc)) {
      const sid = (doc as stores.DocumentSsh).sid;
      const participants = ctx.storeParties.state[sid];
      if (!participants) {
        return true;
      }

      if (participants.length > 1) {
        return window.confirm('Are you sure you want to leave this session?');
      }

      return window.confirm('Are you sure you want to terminate this session?');
    }

    return true;
  }

  return { verifyAndConfirm, hasLastingSshConnection };
}

export default useOnExitConfirmation;
