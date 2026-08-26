/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import {
  UrlIntegrationParams,
  UrlKubeResourcesParams,
  UrlResourcesParams,
} from './config';
import generateResourcePath from './generateResourcePath';

const fullParamPath =
  '/v1/webapi/sites/:clusterId/:name/foo' +
  '?kind=:kind?' +
  '&kinds=:kinds?' +
  '&kubeCluster=:kubeCluster?' +
  '&kubeNamespace=:kubeNamespace?' +
  '&limit=:limit?' +
  '&pinnedOnly=:pinnedOnly?' +
  '&query=:query?' +
  '&resourceType=:resourceType?' +
  '&search=:search?' +
  '&searchAsRoles=:searchAsRoles?' +
  '&sort=:sort?' +
  '&startKey=:startKey?' +
  '&includedResourceMode=:includedResourceMode?' +
  '&regions=:regions?';

test('undefined params are set to empty', () => {
  expect(
    generateResourcePath(fullParamPath, {
      clusterId: 'some-cluster-id',
    })
  ).toStrictEqual(
    '/v1/webapi/sites/some-cluster-id//foo?kind=&kinds=&kubeCluster=&kubeNamespace=&limit=&pinnedOnly=&query=&resourceType=&search=&searchAsRoles=&sort=&startKey=&includedResourceMode=&regions='
  );
});

type allParams = UrlResourcesParams &
  UrlKubeResourcesParams &
  UrlIntegrationParams;

test('defined params are set', () => {
  const urlParams: allParams = {
    includedResourceMode: 'all',
    kind: 'some-kind',
    kinds: ['app', 'db'],
    kubeCluster: 'some-kube-cluster',
    kubeNamespace: 'some-kube-namespace',
    limit: 100,
    name: 'some-name',
    pinnedOnly: true,
    query: 'some-query',
    resourceType: 'some-resource-type',
    search: 'some-search',
    searchAsRoles: 'yes',
    sort: { fieldName: 'sort-field', dir: 'DESC' },
    startKey: 'some-start-key',
    regions: ['us-west-2', 'af-south-1'],
  };
  expect(
    generateResourcePath(fullParamPath, {
      clusterId: 'some-cluster-id',
      ...urlParams,
    })
  ).toStrictEqual(
    '/v1/webapi/sites/some-cluster-id/some-name/foo?kind=some-kind&kinds=app&kinds=db&kubeCluster=some-kube-cluster&kubeNamespace=some-kube-namespace&limit=100&pinnedOnly=true&query=some-query&resourceType=some-resource-type&search=some-search&searchAsRoles=yes&sort=sort-field:desc&startKey=some-start-key&includedResourceMode=all&regions=us-west-2&regions=af-south-1'
  );
});

test('defined params but set to empty values are set to empty', () => {
  const urlParams: allParams = {
    includedResourceMode: null,
    kind: '',
    kinds: [],
    kubeCluster: '',
    kubeNamespace: '',
    limit: 0,
    name: '',
    pinnedOnly: null,
    query: '',
    resourceType: '',
    search: '',
    searchAsRoles: '',
    sort: null,
    startKey: '',
    regions: [],
  };
  expect(
    generateResourcePath(fullParamPath, {
      clusterId: 'some-cluster-id',
      ...urlParams,
    })
  ).toStrictEqual(
    '/v1/webapi/sites/some-cluster-id//foo?kind=&kinds=&kubeCluster=&kubeNamespace=&limit=&pinnedOnly=&query=&resourceType=&search=&searchAsRoles=&sort=&startKey=&includedResourceMode=&regions='
  );
});

test('unknown key values are not set even if declared in path', () => {
  let unknownParamPath = '/v1/webapi/sites/view?foo=:foo?&bar=:bar?&baz=:baz?';
  expect(
    generateResourcePath(unknownParamPath, {
      foo: 'some-foo',
      bar: 'some-bar',
      baz: 'some-baz',
    })
  ).toStrictEqual(unknownParamPath);
});
