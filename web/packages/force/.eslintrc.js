const eslint = require('@gravitational/build/.eslintrc');

// a place for custom eslint rules
const custom = {
  ...eslint,
  rules: {
    ...eslint.rules,
  },
};

module.exports = custom;
