/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

module.exports = {
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 6,
    ecmaFeatures: {
      jsx: true,
      experimentalObjectRestSpread: true,
    },
  },
  env: {
    es6: true,
    browser: true,
    node: true,
  },
  globals: {
    expect: true,
    jest: true,
  },
  ignorePatterns: ['**/dist/**'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:import/errors',
    'plugin:import/warnings',
    'plugin:import/typescript',
  ],
  plugins: ['react', 'babel', 'import', 'react-hooks'],
  overrides: [
    {
      files: ['**/*.test.{ts,tsx,js,jsx}'],
      plugins: ['jest'],
      extends: [
        'plugin:jest/recommended',
        'plugin:testing-library/react',
        'plugin:jest-dom/recommended',
      ],
      rules: {
        'jest/prefer-called-with': 0,
        'jest/prefer-expect-assertions': 0,
        'jest/consistent-test-it': 0,
        'jest/no-try-expect': 0,
        'jest/no-hooks': 0,
        'jest/no-disabled-tests': 0,
        'jest/prefer-strict-equal': 0,
        'jest/prefer-inline-snapshots': 0,
        'jest/require-top-level-describe': 0,
        'jest/no-large-snapshots': ['warn', { maxSize: 200 }],
      },
    },
  ],
  rules: {
    'import/order': [
      'error',
      {
        groups: [
          'builtin',
          'external',
          'internal',
          'parent',
          'sibling',
          'index',
          'object',
          'type',
        ],
        'newlines-between': 'always-and-inside-groups',
      },
    ],
    'no-unused-vars': 'off', // disabled to allow the typescript one to take over and avoid errors in reporting
    '@typescript-eslint/no-unused-vars': ['error'],

    // Severity should be one of the following:
    // "off" or 0 - turn the rule off
    // "warn" or 1 - turn the rule on as a warning (doesn’t affect exit code)
    // "error" or 2 - turn the rule on as an error (exit code is 1 when triggered)

    // <TODO> Enable these rules after fixing all existing issues
    '@typescript-eslint/no-use-before-define': 0,
    '@typescript-eslint/indent': 0,
    '@typescript/no-use-before-define': 0,
    '@typescript-eslint/no-var-requires': 0,
    '@typescript-eslint/camelcase': 0,
    '@typescript-eslint/class-name-casing': 0,
    '@typescript-eslint/no-explicit-any': 0,
    '@typescript-eslint/explicit-function-return-type': 0,
    '@typescript-eslint/explicit-member-accessibility': 0,
    '@typescript-eslint/prefer-interface': 0,
    '@typescript-eslint/no-empty-function': 0,
    '@typescript-eslint/no-this-alias': 0,

    // </TODO>
    'comma-dangle': 0,
    'no-mixed-spaces-and-tabs': 0,
    'no-alert': 0,
    'import/no-named-as-default': 0,
    'import/default': 2,
    // XXX Change to a 2 once e pkg imports are removed from teleterm.
    'import/no-unresolved': 1,
    'no-underscore-dangle': 0,
    'no-case-declarations': 0,
    'prefer-const': 0,
    'no-var': 0,
    'prefer-rest-params': 0,
    'prefer-spread': 0,

    strict: 0,
    'no-console': 1,
    'no-trailing-spaces': 2,
    'react/display-name': 0,
    'react/jsx-no-undef': 2,
    'react/jsx-pascal-case': 2,
    'react/no-danger': 2,
    'react/jsx-no-duplicate-props': 2,
    'react/jsx-sort-prop-types': 0,
    'react/jsx-sort-props': 0,
    'react/jsx-uses-vars': 1,
    'react/no-did-mount-set-state': 1,
    'react/no-did-update-set-state': 1,
    'react/no-unknown-property': 1,
    'react/prop-types': 0,
    'react/self-closing-comp': 0,
    'react/sort-comp': 0,
    'react/jsx-wrap-multilines': 1,
    // allowExpressions allow single expressions in a fragment eg: <>{children}</>
    // https://github.com/jsx-eslint/eslint-plugin-react/blob/f83b38869c7fc2c6a84ef8c2639ac190b8fef74f/docs/rules/jsx-no-useless-fragment.md#allowexpressions
    'react/jsx-no-useless-fragment': [2, { allowExpressions: true }],

    'react-hooks/rules-of-hooks': 1,
    'react-hooks/exhaustive-deps': 1,

    // Turned off because we use automatic runtime.
    'react/jsx-uses-react': 0,
    'react/react-in-jsx-scope': 0,
  },
  settings: {
    react: {
      // React version. "detect" automatically picks the version you have installed.
      version: 'detect',
    },
    'import/resolver': {
      typescript: {},
    },
  },
};
