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
  reactPlugin.configs.flat.recommended,
  importPlugin.flatConfigs.errors,
  importPlugin.flatConfigs.warnings,
  importPlugin.flatConfigs.typescript,
  {
    settings: {
      react: {
        version: 'detect',
      },
    },
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
    },
    plugins: {
      // There is no flat config available.
      'react-hooks': reactHooksPlugin,
    },
    rules: {
      ...reactHooksPlugin.configs.recommended.rules,
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
      // typescript-eslint recommends to turn import/no-unresolved off.
      // https://typescript-eslint.io/troubleshooting/typed-linting/performance/#eslint-plugin-import
      'import/no-unresolved': 'off',
      '@typescript-eslint/no-unused-expressions': [
        'error',
        { allowShortCircuit: true, allowTernary: true, enforceForJSX: true },
      ],
      '@typescript-eslint/no-empty-object-type': [
        'error',
        // with-single-extends is needed to allow for interface extends like we have in jest.d.ts.
        { allowInterfaces: 'with-single-extends' },
      ],

      // <TODO> Enable these recommended typescript-eslint rules after fixing existing issues.
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-this-alias': 'off',
      // </TODO>

      'no-case-declarations': 'off',
      'prefer-const': 'off',
      'no-var': 'off',
      'prefer-rest-params': 'off',

      'no-console': 'warn',
      'no-trailing-spaces': 'error',
      'react/jsx-no-undef': 'error',
      'react/jsx-pascal-case': 'error',
      'react/no-danger': 'error',
      'react/jsx-no-duplicate-props': 'error',
      'react/jsx-sort-prop-types': 'off',
      'react/jsx-sort-props': 'off',
      'react/jsx-uses-vars': 'warn',
      'react/no-did-mount-set-state': 'warn',
      'react/no-did-update-set-state': 'warn',
      'react/no-unknown-property': 'warn',
      'react/prop-types': 'off',
      'react/jsx-wrap-multilines': 'warn',
      // allowExpressions allow single expressions in a fragment eg: <>{children}</>
      // https://github.com/jsx-eslint/eslint-plugin-react/blob/f83b38869c7fc2c6a84ef8c2639ac190b8fef74f/docs/rules/jsx-no-useless-fragment.md#allowexpressions
      'react/jsx-no-useless-fragment': ['error', { allowExpressions: true }],
      'react/display-name': 'off',
      'react/no-children-prop': 'warn',
      'react/no-unescaped-entities': 'warn',
      'react/jsx-key': 'warn',
      'react/jsx-no-target-blank': 'warn',
      // Turned off because we use automatic runtime.
      'react/jsx-uses-react': 'off',
      'react/react-in-jsx-scope': 'off',

      'react-hooks/rules-of-hooks': 'warn',
      'react-hooks/exhaustive-deps': 'warn',
    },
  },
  {
    files: ['**/*.test.{ts,tsx,js,jsx}'],
    languageOptions: {
      globals: globals.jest,
    },
    plugins: {
      jest: jestPlugin,
      'testing-library': testingLibraryPlugin,
      'jest-dom': jestDomPlugin,
    },
    rules: {
      ...jestPlugin.configs['flat/recommended'].rules,
      ...testingLibraryPlugin.configs['flat/react'].rules,
      ...jestDomPlugin.configs['flat/recommended'].rules,
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
    // Allow require imports in .js files, as migrating our project to ESM modules requires a lot of
    // changes.
    files: ['**/*.js'],
    rules: {
      '@typescript-eslint/no-require-imports': 'warn',
    },
  }
);
