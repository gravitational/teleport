const config = require('@gravitational/build/jest/config');
module.exports = {
  ...config,
  collectCoverageFrom: [
    // comment out until shared directory is finished testing
    // '**/packages/design/src/**/*.jsx',
    '**/packages/shared/components/**/*.jsx',
  ],
  coverageReporters: ['text-summary', 'lcov'],
};
