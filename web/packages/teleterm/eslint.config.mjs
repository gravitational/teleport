import base from '@gravitational/build/eslint.config.mjs';

export default [
  ...base,
  {
    rules: {
      'no-console': 'off',
    },
  },
];
