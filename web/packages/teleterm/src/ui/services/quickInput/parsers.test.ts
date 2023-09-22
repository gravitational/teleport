/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { QuickSshLoginSuggester, QuickServerSuggester } from './suggesters';

// Jest doesn't let us selectively automock classes. See https://github.com/facebook/jest/issues/11995
//
// So instead for now we just mock all classes in the module and then do `jest.requireActual` when
// we need to have the actual class when writing tests for it.
jest.mock('./parsers');

afterEach(() => {
  jest.restoreAllMocks();
});

test("tsh ssh picker returns unknown command if it's missing the first positional arg", async () => {
  const QuickSshLoginSuggesterMock = QuickSshLoginSuggester as jest.MockedClass<
    typeof QuickSshLoginSuggester
  >;
  const QuickServerSuggesterMock = QuickServerSuggester as jest.MockedClass<
    typeof QuickServerSuggester
  >;
  const ActualQuickTshSshParser =
    jest.requireActual('./parsers').QuickTshSshParser;

  const parser = new ActualQuickTshSshParser(
    new QuickSshLoginSuggesterMock(undefined, undefined),
    new QuickServerSuggesterMock(undefined, undefined)
  );

  const emptyInput = await parser.parse('', 0);
  expect(emptyInput.command).toEqual({ kind: 'command.unknown' });

  const whitespace = await parser.parse(' ', 0);
  expect(whitespace.command).toEqual({ kind: 'command.unknown' });
});

test('tsh ssh picker returns unknown command if the input includes any additional flags', async () => {
  const QuickSshLoginSuggesterMock = QuickSshLoginSuggester as jest.MockedClass<
    typeof QuickSshLoginSuggester
  >;
  const QuickServerSuggesterMock = QuickServerSuggester as jest.MockedClass<
    typeof QuickServerSuggester
  >;
  const ActualQuickTshSshParser =
    jest.requireActual('./parsers').QuickTshSshParser;

  const parser = new ActualQuickTshSshParser(
    new QuickSshLoginSuggesterMock(undefined, undefined),
    new QuickServerSuggesterMock(undefined, undefined)
  );

  const fullFlagBefore = await parser.parse('--foo user@node', 0);
  expect(fullFlagBefore.command).toEqual({ kind: 'command.unknown' });

  const shortFlagBefore = await parser.parse('-p 22 user@node', 0);
  expect(shortFlagBefore.command).toEqual({ kind: 'command.unknown' });

  const commandAfter = await parser.parse('user@node ls', 0);
  expect(commandAfter.command).toEqual({ kind: 'command.unknown' });
});
