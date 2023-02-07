const config = require('@gravitational/build/jest/config');

process.env.TZ = 'UTC';

/** @type {import('@jest/types').Config.InitialOptions} */
module.exports = {
  ...config,
  globals: {
    electron: {},
  },
  testPathIgnorePatterns: [
    // Skipped until e files are removed from teleterm.
    'web/packages/teleterm/src/ui/TabHost/TabHost.test.tsx',
  ],
  collectCoverageFrom: [
    // comment out until shared directory is finished testing
    // '**/packages/design/src/**/*.jsx',
    '**/packages/shared/components/**/*.jsx',
  ],
  coverageReporters: ['text-summary', 'lcov'],
};
