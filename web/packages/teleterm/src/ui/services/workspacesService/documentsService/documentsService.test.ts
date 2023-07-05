/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { ImmutableStore } from 'teleterm/ui/services/immutableStore';

import { DocumentsService } from './documentsService';
import { Document, DocumentGateway, DocumentTshNode } from './types';

function getMockedDocuments(): Document[] {
  return [
    { uri: '/docs/test1', kind: 'doc.terminal_shell', title: 'T1' },
    { uri: '/docs/test2', kind: 'doc.terminal_shell', title: 'T2' },
    { uri: '/docs/test3', kind: 'doc.terminal_shell', title: 'T3' },
  ];
}

function createService(mockDocks: Document[]): DocumentsService {
  const store = new ImmutableStore<{
    documents: Document[];
    location: string;
  }>();
  store.state = { documents: [], location: undefined };
  const service = new DocumentsService(
    () => store.state,
    draftState => store.setState(draftState)
  );
  mockDocks.forEach(d => service.add(d));

  return service;
}

test('open the document', () => {
  const mockedDocuments = getMockedDocuments();
  const service = createService(mockedDocuments);

  service.open(mockedDocuments[0].uri);

  expect(service.getActive()).toStrictEqual(mockedDocuments[0]);
});

test('get documents should return all documents', () => {
  const mockedDocuments = getMockedDocuments();
  const service = createService(mockedDocuments);

  expect(service.getDocuments()).toStrictEqual(mockedDocuments);
});

test('get document should return the document', () => {
  const mockedDocuments = getMockedDocuments();
  const service = createService(mockedDocuments);

  expect(service.getDocument(mockedDocuments[0].uri)).toStrictEqual(
    mockedDocuments[0]
  );
});

describe('document should be added', () => {
  const mockedDocuments = getMockedDocuments();
  const newDocument: DocumentGateway = {
    uri: '/docs/new-doc',
    title: 'New doc',
    kind: 'doc.gateway',
    gatewayUri: '/gateways/123',
    targetUri: '/clusters/bar/dbs/quux',
    targetName: 'quux',
    targetUser: 'foo',
    origin: 'resource_table',
  };

  test('at the specific position', () => {
    const service = createService(mockedDocuments);

    service.add(newDocument);

    expect(
      service.getDocuments()[service.getDocuments().length - 1]
    ).toStrictEqual(newDocument);
  });

  test('at the end if position is not specified', () => {
    const service = createService(mockedDocuments);
    const newPosition = 2;

    service.add(newDocument, newPosition);

    expect(service.getDocuments()[newPosition]).toStrictEqual(newDocument);
  });
});

test('update the document', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);
  const newTitle = 'A new title!';
  service.update(mockedDocks[0].uri, { title: newTitle });

  expect(service.getDocument(mockedDocks[0].uri).title).toBe(newTitle);
});

test('filter should omit given document', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);

  expect(service.filter(mockedDocks[0].uri)).toStrictEqual(
    service.getDocuments().filter(d => d.uri !== mockedDocks[0].uri)
  );
});

test('only TSH node documents should be returned', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);

  const tshNodeDocument: DocumentTshNode = {
    uri: '/docs/test1',
    kind: 'doc.terminal_tsh_node',
    title: 'TSH',
    serverId: 'bar',
    login: '',
    serverUri: '/clusters/foo/servers/bar',
    status: 'connecting',
    rootClusterId: '',
    leafClusterId: undefined,
    origin: 'resource_table',
  };

  service.add(tshNodeDocument);

  expect(service.getTshNodeDocuments()).toStrictEqual([tshNodeDocument]);
});

test('only gateway documents should be returned', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);

  const gatewayDocument: DocumentGateway = {
    uri: '/docs/test1',
    kind: 'doc.gateway',
    title: 'gw',
    gatewayUri: '/gateways/123',
    targetUri: '/clusters/bar/dbs/quux',
    targetName: 'quux',
    targetUser: 'foo',
    origin: 'resource_table',
  };

  service.add(gatewayDocument);

  expect(service.getGatewayDocuments()).toStrictEqual([gatewayDocument]);
});

describe('next URI', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);

  test('should be next element URI if given URI is not last', () => {
    expect(service.getNextUri(mockedDocks[1].uri)).toBe(mockedDocks[2].uri);
  });

  test('should be previous element URI if given URI is last', () => {
    expect(service.getNextUri(mockedDocks[mockedDocks.length - 1].uri)).toBe(
      mockedDocks[mockedDocks.length - 2].uri
    );
  });
});

test('close other docs', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);

  service.closeOthers(mockedDocks[0].uri);

  expect(service.getDocuments()).toContain(mockedDocks[0]);
  expect(service.getDocuments()).not.toContain(mockedDocks[1]);
  expect(service.getDocuments()).not.toContain(mockedDocks[2]);
});

test('close docs to the right', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);

  service.closeToRight(mockedDocks[1].uri);

  expect(service.getDocuments()).toContain(mockedDocks[0]);
  expect(service.getDocuments()).toContain(mockedDocks[1]);
  expect(service.getDocuments()).not.toContain(mockedDocks[2]);
});

test('duplicate PTY doc and activate it', () => {
  const mockedDocks = getMockedDocuments();
  const service = createService(mockedDocks);
  const ptyToDuplicate = mockedDocks[1];
  const ptyToDuplicateIndex = service.getDocuments().indexOf(ptyToDuplicate);
  const initialLength = service.getDocuments().length;

  service.duplicatePtyAndActivate(ptyToDuplicate.uri);

  expect(service.getDocuments()).toHaveLength(initialLength + 1);
  expect({
    ...service.getDocuments()[ptyToDuplicateIndex + 1],
    uri: '', // omit URI, all other properties should be copied
  }).toStrictEqual({ ...ptyToDuplicate, uri: '' });
  expect(service.getActive()).toStrictEqual(
    service.getDocuments()[ptyToDuplicateIndex + 1]
  );
});
