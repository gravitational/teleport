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

import { getPlatformType } from 'design/platform';

import ConsoleContext from './consoleContext';

/**
 * getMappedAction maps keyboard hot keys pressed to the ones
 * that this app recognizes.
 *
 * Registered so far:
 * - Tab switch:
 *   - windows/ubuntu: alt + <1-9>
 *   - mac: ctrl + <1-9>
 *
 * @param event reference to the event object
 */
export function getMappedAction(event) {
  // 1-9 defines the event.key's on the keyboard numbers
  const index = ['1', '2', '3', '4', '5', '6', '7', '8', '9'].indexOf(
    event.key
  );

  const { isMac } = getPlatformType();
  const isModifierKey = (isMac && event.ctrlKey) || event.altKey;

  let tabSwitch = undefined;
  if (isModifierKey && index !== -1) {
    tabSwitch = { index };
  }

  return { tabSwitch };
}

/**
 * useKeyboardNav registers handlers for handling hot key events.
 *
 * @param ctx data that is shared between Console related components
 */
const useKeyboardNav = (ctx: ConsoleContext) => {
  useEffect(() => {
    const handleKeydown = event => {
      const { tabSwitch } = getMappedAction(event);
      if (!tabSwitch) {
        return;
      }

      event.preventDefault();
      const doc = ctx.getDocuments()[tabSwitch.index + 1];
      if (doc) {
        ctx.gotoTab(doc);
      }
    };

    window.addEventListener('keydown', handleKeydown);
    return () => window.removeEventListener('keydown', handleKeydown);
  }, []);
};

export default useKeyboardNav;
