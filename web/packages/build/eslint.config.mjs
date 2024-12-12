/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';
import reactPlugin from 'eslint-plugin-react';
import reactHooksPlugin from 'eslint-plugin-react-hooks';
import importPlugin from 'eslint-plugin-import';
import jestPlugin from 'eslint-plugin-jest';
import testingLibraryPlugin from 'eslint-plugin-testing-library';
import jestDomPlugin from 'eslint-plugin-jest-dom';
import globals from 'globals';

export default tseslint.config(
  {
    ignores: [
      '**/dist/**',
      '**/*_pb.*',
      '.eslintrc.js',
      '**/tshd/**/*_pb.js',
      // WASM generated files
      'web/packages/teleport/src/ironrdp/pkg',
      'web/packages/teleterm/build',
    ],
  },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.test.{ts,tsx,js,jsx}'],
    languageOptions: {
      globals: {
        ...globals.jest,
      },
    },
    plugins: {
      jest: jestPlugin,
      'testing-library': testingLibraryPlugin,
      'jest-dom': jestDomPlugin,
    },
    rules: {
      'jest/prefer-called-with': 'off',
      'jest/prefer-expect-assertions': 'off',
      'jest/consistent-test-it': 'off',
      'jest/no-try-expect': 'off',
      'jest/no-hooks': 'off',
      'jest/no-disabled-tests': 'off',
      'jest/prefer-strict-equal': 'off',
      'jest/prefer-inline-snapshots': 'off',
      'jest/require-top-level-describe': 'off',
      'jest/no-large-snapshots': ['warn', { maxSize: 200 }],
    },
  },
  {
    files: ['**/*.js'],
    rules: {
      '@typescript-eslint/no-require-imports': 'warn',
    },
  },
  {
    languageOptions: {
      ecmaVersion: 6,
      sourceType: 'module',
      globals: {
        ...globals.browser,
        ...globals.node,
        expect: 'readonly',
        jest: 'readonly',
      },
      parser: tseslint.parser,
      parserOptions: {
        ecmaFeatures: {
          jsx: true,
        },
      },
    },
    plugins: {
      '@typescript-eslint': tseslint.plugin,
      react: reactPlugin,
      'react-hooks': reactHooksPlugin,
      import: importPlugin,
    },
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

      '@typescript-eslint/no-unused-vars': ['error'],
      '@typescript-eslint/no-unused-expressions': [
        'error',
        { allowShortCircuit: true, allowTernary: true, enforceForJSX: true },
      ],
      '@typescript-eslint/no-empty-object-type': [
        'error',
        { allowInterfaces: 'with-single-extends' },
      ],

      '@typescript-eslint/no-use-before-define': 'off',
      '@typescript-eslint/indent': 'off',
      '@typescript/no-use-before-define': 'off',
      '@typescript-eslint/no-var-requires': 'off',
      '@typescript-eslint/camelcase': 'off',
      '@typescript-eslint/class-name-casing': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/explicit-function-return-type': 'off',
      '@typescript-eslint/explicit-member-accessibility': 'off',
      '@typescript-eslint/prefer-interface': 'off',
      '@typescript-eslint/no-empty-function': 'off',
      '@typescript-eslint/no-this-alias': 'off',

      'react/jsx-no-undef': 'error',
      'react/jsx-pascal-case': 'error',
      'react/no-danger': 'error',
      'react/display-name': 'off',
      'react/jsx-no-duplicate-props': 'error',
      'react/jsx-no-useless-fragment': ['error', { allowExpressions: true }],
      'react-hooks/rules-of-hooks': 'warn',
      'react-hooks/exhaustive-deps': 'warn',
      'react/jsx-sort-prop-types': 'off',
      'react/jsx-sort-props': 'off',
      'react/jsx-uses-vars': 'warn',
      'react/no-did-mount-set-state': 'warn',
      'react/react-in-jsx-scope': 'off',
      'react/no-did-update-set-state': 'warn',
      'react/no-unknown-property': 'warn',
      'react/prop-types': 'off',
      'react/self-closing-comp': 'off',
      'react/sort-comp': 'off',
      'react/jsx-wrap-multilines': 'warn',

      'comma-dangle': 'off',
      'no-mixed-spaces-and-tabs': 'off',
      'no-alert': 'off',
      'import/no-named-as-default': 'off',
      'import/no-unresolved': 'off',
      'import/default': 'error',
      'no-underscore-dangle': 'off',
      'no-case-declarations': 'off',
      'prefer-const': 'off',
      'no-var': 'off',
      'prefer-rest-params': 'off',
      'prefer-spread': 'off',
      'no-console': 'warn',
      'no-trailing-spaces': 'error',
    },
  }
);
