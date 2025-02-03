const config = require('@gravitational/build/jest/config');

process.env.TZ = 'UTC';

const esModules = [
  'strip-ansi',
  'ansi-regex',
  'd3-color',
  'd3-scale',
  'd3-interpolate',
  'd3-time-format',
  'd3-array',
  'd3-format',
  'd3-time',
  'd3-shape',
  'd3-path',
  'internmap',
  '@nivo/bar',
].join('|');

/** @type {import('@jest/types').Config.InitialOptions} */
module.exports = {
  ...config,
  globals: {
    electron: {},
  },
  collectCoverageFrom: [
    // comment out until shared directory is finished testing
    // '**/packages/design/src/**/*.jsx',
    '**/packages/shared/components/**/*.jsx',
  ],
  // Copied from https://jestjs.io/docs/configuration#transformignorepatterns-arraystring
  // Because in pnpm packages are symlinked to node_modules/.pnpm,
  // we need to transform packages in that directory.
  // We use a relative pattern to match the second 'node_modules/' in
  // 'node_modules/.pnpm/@scope+pkg-b@x.x.x/node_modules/@scope/pkg-b/'.
  transformIgnorePatterns: [`node_modules/(?!.pnpm|${esModules})`],
  coverageReporters: ['text-summary', 'lcov'],
  testPathIgnorePatterns: [
    'e2e',
    'docs/check-redirects',
    // This is necessary, as this file may be recreated during tests. If we
    // don't ignore it, `pnpm tdd` may enter a recreate-and-rerun loop.
    '<rootDir>/tmp/preset-roles.json',
  ],
  testEnvironmentOptions: {
    customExportConditions: [''],
  },
  setupFilesAfterEnv: [
    '<rootDir>/web/packages/build/jest/setupTests.ts',
    '<rootDir>/web/packages/build/jest/customMatchers.ts',
  ],
};
