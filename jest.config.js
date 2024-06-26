const config = require('@gravitational/build/jest/config');

process.env.TZ = 'UTC';

const esModules = [
  'strip-ansi',
  'ansi-regex',
  'd3-color',
  'd3-scale',
  'd3-interpolate',
  'd3-array',
  'd3-format',
  'd3-time',
  'd3-shape',
  'd3-path',
  'internmap',
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
  transformIgnorePatterns: [`/node_modules/(?!${esModules})`],
  coverageReporters: ['text-summary', 'lcov'],
  testPathIgnorePatterns: ['e2e'],
  setupFilesAfterEnv: [
    '<rootDir>/web/packages/shared/setupTests.tsx',
    '<rootDir>/web/packages/build/jest/customMatchers.ts',
  ],
};
