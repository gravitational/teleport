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

import renderHook from 'design/utils/renderHook';

import session from 'teleport/services/websession';

import ConsoleContext from './consoleContext';
import useOnExitConfirmation from './useOnExitConfirmation';

test('confirmation dialog before terminating an active ssh session', () => {
  const ctx = new ConsoleContext();
  const { current } = renderHook(() => useOnExitConfirmation(ctx));
  const event = new Event('beforeunload');

  // two prompts that can be called before closing session/window
  jest.spyOn(window, 'confirm').mockReturnValue(false);
  jest.spyOn(event, 'preventDefault');

  ctx.storeDocs.add({
    kind: 'nodes',
    url: 'test',
    created: new Date(),
  });
  let docs = ctx.getDocuments();

  // test blank doc does not call prompt
  const docBlank = docs[0];
  let retVal = current.verifyAndConfirm(docBlank);
  expect(retVal).toBe(true);
  expect(window.confirm).not.toHaveBeenCalled();

  // test nodes doc does not call prompt
  const docNode = docs[1];
  retVal = current.verifyAndConfirm(docNode);
  expect(retVal).toBe(true);
  expect(window.confirm).not.toHaveBeenCalled();

  // test blank and node doc, does not trigger prompt
  window.dispatchEvent(event);
  expect(event.preventDefault).not.toHaveBeenCalled();

  // add a new (just created) ssh doc
  ctx.storeDocs.add({
    kind: 'terminal',
    status: 'connected',
    url: 'localhost',
    serverId: 'serverId',
    login: 'login',
    created: new Date(),
    sid: 'random-123-sid',
    latency: {
      client: 0,
      server: 0,
    },
  });
  docs = ctx.getDocuments();

  // test new terminal doc does not call prompt
  const docTerminal = docs[2];
  retVal = current.verifyAndConfirm(docTerminal);
  expect(retVal).toBe(true);
  expect(window.confirm).not.toHaveBeenCalled();

  // test new terminal doc does not trigger prompt
  window.dispatchEvent(event);
  expect(event.preventDefault).not.toHaveBeenCalled();

  // change date to an old date
  docTerminal.created = new Date('2019-04-01');

  // test that expired session does not prompt
  jest.spyOn(session, '_timeLeft').mockReturnValue(0);
  window.dispatchEvent(event);
  expect(event.preventDefault).not.toHaveBeenCalled();

  // test aged terminal doc calls prompt
  ctx.storeParties.setParties({ 'random-123-sid': [] });
  retVal = current.verifyAndConfirm(docTerminal);
  expect(retVal).toBe(false);
  expect(window.confirm).toHaveReturnedWith(false);

  // test aged terminal doc triggers prompt
  jest.spyOn(session, '_timeLeft').mockReturnValue(5);
  window.dispatchEvent(event);
  expect(event.preventDefault).toHaveBeenCalledTimes(1);
});
