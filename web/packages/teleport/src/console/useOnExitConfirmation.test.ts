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

import renderHook from 'design/utils/renderHook';
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

  // test aged terminal doc calls prompt
  retVal = current.verifyAndConfirm(docTerminal);
  expect(retVal).toBe(false);
  expect(window.confirm).toHaveReturnedWith(false);

  // test aged terminal doc triggers prompt
  window.dispatchEvent(event);
  expect(event.preventDefault).toHaveBeenCalledTimes(1);
});
