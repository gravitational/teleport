const config = require('@gravitational/build/jest/config');
module.exports = {
  ...config,
  collectCoverageFrom: ['**/packages/design/src/**/*.jsx'],
  coverageReporters: ['text-summary', 'lcov'],
};
