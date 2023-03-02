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

import { Params, routing } from './uri';

const getServerUriTests: Array<
  { name: string; input: Params } & (
    | { output: string; wantErr?: never }
    | { wantErr: any; output?: never }
  )
> = [
  {
    name: 'returns a server URI for a root cluster',
    input: { rootClusterId: 'foo', serverId: 'ubuntu' },
    output: '/clusters/foo/servers/ubuntu',
  },
  {
    name: 'returns a server URI for a leaf cluster',
    input: { rootClusterId: 'foo', leafClusterId: 'bar', serverId: 'ubuntu' },
    output: '/clusters/foo/leaves/bar/servers/ubuntu',
  },
  {
    name: 'throws an error if serverId is missing from the root cluster URI',
    input: { rootClusterId: 'foo' },
    wantErr: new TypeError('Expected "serverId" to be defined'),
  },
  {
    name: 'throws an error if serverId is missing from the leaf cluster URI',
    input: { rootClusterId: 'foo', leafClusterId: 'bar' },
    wantErr: new TypeError('Expected "serverId" to be defined'),
  },
  {
    // This isn't necessarily a behavior which we should depend on, but we should document it
    // nonetheless.
    name: 'returns a server URI if extra params are included',
    input: { rootClusterId: 'foo', serverId: 'ubuntu', dbId: 'postgres' },
    output: '/clusters/foo/servers/ubuntu',
  },
];
/* eslint-disable jest/no-conditional-expect */
test.each(getServerUriTests)(
  'getServerUri $name',
  ({ input, output, wantErr }) => {
    if (wantErr) {
      expect(() => routing.getServerUri(input)).toThrow(wantErr);
    } else {
      expect(routing.getServerUri(input)).toEqual(output);
    }
  }
);
/* eslint-enable jest/no-conditional-expect */
