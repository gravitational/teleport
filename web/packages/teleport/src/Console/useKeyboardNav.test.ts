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

import { Document } from 'teleport/Console/stores';

import ConsoleContext from './consoleContext';
import useKeyboardNav from './useKeyboardNav';

test('keyboard press is respected', () => {
  const ctx = new ConsoleContext();
  renderHook(() => useKeyboardNav(ctx));

  jest.spyOn(ctx, 'gotoTab').mockImplementation(({ url }) => {
    return url;
  });

  // test nonexistent tab
  let event = new KeyboardEvent('keydown', { key: '1', altKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).not.toHaveBeenCalled();

  // add a few tabs
  const tab1 = tabGenerator('tab1');
  const tab2 = tabGenerator('tab2');
  const tab3 = tabGenerator('tab3');
  const tab4 = tabGenerator('tab4');
  const tab5 = tabGenerator('tab5');
  const tab6 = tabGenerator('tab6');
  const tab7 = tabGenerator('tab7');
  const tab8 = tabGenerator('tab8');
  const tab9 = tabGenerator('tab9');

  ctx.storeDocs.add({ ...tab1 });
  ctx.storeDocs.add({ ...tab2 });
  ctx.storeDocs.add({ ...tab3 });
  ctx.storeDocs.add({ ...tab4 });

  // test nonexistent tab
  event = new KeyboardEvent('keydown', { key: '5', altKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).not.toHaveBeenCalled();

  ctx.storeDocs.add({ ...tab5 });
  ctx.storeDocs.add({ ...tab6 });
  ctx.storeDocs.add({ ...tab7 });
  ctx.storeDocs.add({ ...tab8 });
  ctx.storeDocs.add({ ...tab9 });

  // test correct tabs was called
  event = new KeyboardEvent('keydown', { key: '4', altKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab4.url);

  event = new KeyboardEvent('keydown', { key: '6', altKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab6.url);

  event = new KeyboardEvent('keydown', { key: '1', altKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab1.url);

  event = new KeyboardEvent('keydown', { key: '9', altKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab9.url);

  jest.clearAllMocks();

  // test non-mac platform doesn't trigger end event with ctrl + num
  event = new KeyboardEvent('keydown', { key: '4', ctrlKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).not.toHaveBeenCalled();

  // set platform to mac
  jest.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue('Macintosh');

  // test key combo not handled on mac platform
  event = new KeyboardEvent('keydown', { key: '0', ctrlKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).not.toHaveBeenCalled();

  // test correct tabs was called on mac platform
  event = new KeyboardEvent('keydown', { key: '4', ctrlKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab4.url);

  event = new KeyboardEvent('keydown', { key: '6', ctrlKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab6.url);

  event = new KeyboardEvent('keydown', { key: '1', ctrlKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab1.url);

  event = new KeyboardEvent('keydown', { key: '9', ctrlKey: true });
  window.dispatchEvent(event);
  expect(ctx.gotoTab).toHaveReturnedWith(tab9.url);
});

const tabGenerator = tab => {
  return {
    kind: 'terminal',
    url: tab,
    created: null,
    status: null,
    login: null,
    serverId: null,
  } as Document;
};
