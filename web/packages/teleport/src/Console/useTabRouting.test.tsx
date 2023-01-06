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
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import renderHook from 'design/utils/renderHook';

import useTabRouting from './useTabRouting';
import ConsoleContext from './consoleContext';

test('handling of index route', async () => {
  const ctx = new ConsoleContext();
  const wrapper = makeWrapper('/web/cluster/localhost/console');
  const { current } = renderHook(() => useTabRouting(ctx), { wrapper });
  const docs = ctx.getDocuments();
  const blank = docs[0];
  expect(blank.kind).toBe('blank');
  expect(docs).toHaveLength(1);
  expect(current.activeDocId).toBe(blank.id);
});

test('handling of cluster nodes route', async () => {
  const ctx = new ConsoleContext();
  const wrapper = makeWrapper('/web/cluster/localhost/console/nodes');
  const { current } = renderHook(() => useTabRouting(ctx), { wrapper });
  const docs = ctx.getDocuments();
  const activeDoc = docs.find(d => d.id === current.activeDocId);
  expect(activeDoc.kind).toBe('nodes');
  expect(docs).toHaveLength(2);
});

test('handling of init ssh session route', async () => {
  const ctx = new ConsoleContext();
  const wrapper = makeWrapper('/web/cluster/localhost/console/node/one/root');
  const { current } = renderHook(() => useTabRouting(ctx), { wrapper });
  const docs = ctx.getDocuments();
  expect(docs[1].kind).toBe('terminal');
  expect(docs[1].id).toBe(current.activeDocId);
  expect(docs).toHaveLength(2);
});

test('handling of join ssh session route', async () => {
  const ctx = new ConsoleContext();
  const wrapper = makeWrapper('/web/cluster/localhost/console/session/123');
  const { current } = renderHook(() => useTabRouting(ctx), { wrapper });
  const docs = ctx.getDocuments();
  expect(docs[1].kind).toBe('terminal');
  expect(docs[1].id).toBe(current.activeDocId);
  expect(docs).toHaveLength(2);
});

test('active document id', async () => {
  const ctx = new ConsoleContext();
  const doc = ctx.addSshDocument({
    login: 'root',
    serverId: 'server-123',
    clusterId: 'two',
  });

  const countBefore = ctx.getDocuments();
  const wrapper = makeWrapper('/web/cluster/two/console/node/server-123/root');
  const { current } = renderHook(() => useTabRouting(ctx), { wrapper });
  const countAfter = ctx.getDocuments();
  expect(doc.id).toBe(current.activeDocId);
  expect(countAfter).toBe(countBefore);
});

function makeWrapper(route: string) {
  return function MockedContextProviders(props: any) {
    const history = createMemoryHistory({
      initialEntries: [route],
    });

    return <Router history={history} {...props} />;
  };
}
