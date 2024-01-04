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
