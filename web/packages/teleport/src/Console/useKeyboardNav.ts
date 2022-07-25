/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { getPlatform } from 'design/theme/utils';

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

  const { isMac } = getPlatform();
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
  React.useEffect(() => {
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
