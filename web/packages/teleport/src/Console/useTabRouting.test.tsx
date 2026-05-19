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

import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import renderHook from 'design/utils/renderHook';

import ConsoleContext from './consoleContext';
import useTabRouting from './useTabRouting';

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

test('handling of init kubeExec session route', async () => {
  const ctx = new ConsoleContext();
  const wrapper = makeWrapper(
    '/web/cluster/localhost/console/kube/exec/kubeCluster/'
  );
  const { current } = renderHook(() => useTabRouting(ctx), { wrapper });
  const docs = ctx.getDocuments();
  expect(docs[1].kind).toBe('kubeExec');
  expect(docs[1].id).toBe(current.activeDocId);
  expect(docs).toHaveLength(2);
});

test('handling of init kubeExec session route with container', async () => {
  const ctx = new ConsoleContext();
  const wrapper = makeWrapper(
    '/web/cluster/localhost/console/kube/exec/kubeCluster/'
  );
  const { current } = renderHook(() => useTabRouting(ctx), { wrapper });
  const docs = ctx.getDocuments();
  expect(docs[1].kind).toBe('kubeExec');
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

test('active document id, document url with query parameters', async () => {
  const ctx = new ConsoleContext();
  const doc = ctx.addKubeExecDocument({
    clusterId: 'cluster1',
    kubeId: 'kube1',
  });

  const countBefore = ctx.getDocuments();
  const wrapper = makeWrapper('/web/cluster/cluster1/console/kube/exec/kube1/');
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
