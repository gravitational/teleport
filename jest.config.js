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
  // https://jestjs.io/docs/configuration#transformignorepatterns-arraystring
  transformIgnorePatterns: [`node_modules/(?!.pnpm|${esModules})`],
  coverageReporters: ['text-summary', 'lcov'],
  testPathIgnorePatterns: ['e2e'],
  setupFilesAfterEnv: [
    '<rootDir>/web/packages/build/jest/setupTests.ts',
    '<rootDir>/web/packages/build/jest/customMatchers.ts',
  ],
};
