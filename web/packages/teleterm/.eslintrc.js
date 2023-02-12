const eslintConfig = require('@gravitational/build/.eslintrc');

eslintConfig.ignorePatterns = ['**/tshd/**/*_pb.js'];
eslintConfig.rules['no-console'] = 'off';

module.exports = eslintConfig;
