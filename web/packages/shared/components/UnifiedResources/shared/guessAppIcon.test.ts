import { App } from 'teleport/services/apps';
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
];

test.each(testCases)('guessAppIcon: $name', ({ app, expectedIcon }) => {
  expect(guessAppIcon(app)).toEqual(expectedIcon);
});
