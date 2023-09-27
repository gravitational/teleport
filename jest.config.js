const config = require('@gravitational/build/jest/config');

process.env.TZ = 'UTC';

const esModules = ['strip-ansi-stream', 'ansi-regex'].join('|');

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
  setupFilesAfterEnv: ['<rootDir>/web/packages/shared/setupTests.tsx'],
  testPathIgnorePatterns: ["e2e"],
};
