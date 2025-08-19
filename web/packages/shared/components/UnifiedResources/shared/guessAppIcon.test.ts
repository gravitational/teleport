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

// eslint-disable-next-line no-restricted-imports -- FIXME
import { App } from 'teleport/services/apps';
// eslint-disable-next-line no-restricted-imports -- FIXME
import makeApp from 'teleport/services/apps/makeApps';

import { guessAppIcon } from './guessAppIcon';

const testCases: { name: string; app: App; expectedIcon: string }[] = [
  {
    name: 'match by exact name',
    app: makeApp({ name: 'Google Analytics' }),
    expectedIcon: 'googleanalytics',
  },
  {
    name: 'match by name',
    app: makeApp({ name: 'something Google in between Analytics' }),
    expectedIcon: 'googleanalytics',
  },
  {
    name: 'match with dashes in name',
    app: makeApp({ name: 'adobe-marketo' }),
    expectedIcon: 'adobemarketo',
  },
  {
    name: 'match by exact friendly name',
    app: makeApp({
      name: 'no-match',
      friendlyName: '1Password',
    }),
    expectedIcon: '1password',
  },
  {
    name: 'match by friendly name',
    app: makeApp({
      name: 'no-match',
      friendlyName: 'Dev 1 Password',
    }),
    expectedIcon: '1password',
  },
  {
    name: 'match by label',
    app: makeApp({
      name: 'no-match',
      labels: [
        { name: 'mode', value: 'testing' },
        { name: 'env', value: 'dev' },
        { name: 'teleport.icon', value: 'outreach.io' },
      ],
    }),
    expectedIcon: 'outreach.io',
  },
  {
    name: 'match by label - default value',
    app: makeApp({
      name: 'no-match',
      labels: [{ name: 'teleport.icon', value: 'default' }],
    }),
    expectedIcon: 'application',
  },
  {
    name: 'no matches',
    app: makeApp({
      name: 'no-match',
    }),
    expectedIcon: 'application',
  },
  {
    name: 'generic match, if exact sub brand does not match',
    app: makeApp({
      name: 'Something MicroSoft and stuff',
    }),
    expectedIcon: 'microsoft',
  },
  {
    name: 'match by name with paranthesis and brackets',
    app: makeApp({
      name: 'Clearfeed (adobe) [adobe]',
    }),
    expectedIcon: 'clearfeed',
  },
  {
    name: 'match by name if whole text starts and ends with paranthesis',
    app: makeApp({
      name: '(clearfeed)',
    }),
    expectedIcon: 'clearfeed',
  },
  {
    name: 'match by name if whole text starts and ends with bracket',
    app: makeApp({
      name: '[clearfeed]',
    }),
    expectedIcon: 'clearfeed',
  },
];

test.each(testCases)('guessAppIcon: $name', ({ app, expectedIcon }) => {
  expect(guessAppIcon(app)).toEqual(expectedIcon);
});
