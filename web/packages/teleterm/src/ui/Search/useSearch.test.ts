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

import { sortResults } from './useSearch';
import {
  makeResourceResult,
  makeServer,
  makeKube,
  makeLabelsList,
} from './searchResultTestHelpers';

describe('sortResults', () => {
  it('uses the displayed resource name as the tie breaker if the scores are equal', () => {
    const server = makeResourceResult({
      kind: 'server',
      resource: makeServer({ hostname: 'z' }),
    });
    const kube = makeResourceResult({
      kind: 'kube',
      resource: makeKube({ name: 'a' }),
    });
    const sortedResults = sortResults([server, kube], '');

    expect(sortedResults[0]).toEqual(kube);
    expect(sortedResults[1]).toEqual(server);
  });

  it('saves individual label match scores', () => {
    const server = makeResourceResult({
      kind: 'server',
      resource: makeServer({
        labelsList: makeLabelsList({ quux: 'bar-baz', foo: 'bar' }),
      }),
    });

    const { labelMatches } = sortResults([server], 'foo bar')[0];

    labelMatches.forEach(match => {
      expect(match.score).toBeGreaterThan(0);
    });

    const quuxMatches = labelMatches.filter(
      match => match.labelName === 'quux'
    );
    const quuxMatch = quuxMatches[0];
    const fooMatches = labelMatches.filter(match => match.labelName === 'foo');

    expect(quuxMatches).toHaveLength(1);
    expect(fooMatches).toHaveLength(2);
    expect(fooMatches[0].score).toBeGreaterThan(quuxMatch.score);
    expect(fooMatches[1].score).toBeGreaterThan(quuxMatch.score);
  });
});
