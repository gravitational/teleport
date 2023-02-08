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

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { CommandLauncher } from './commandLauncher';

it('returns tsh install & uninstall autocomplete command on macOS', () => {
  const appContext = new MockAppContext({ platform: 'darwin' });
  const commandLauncher = new CommandLauncher(appContext);
  const autocompleteCommandNames = commandLauncher
    .getAutocompleteCommands()
    .map(c => c.displayName);

  expect(autocompleteCommandNames).toContain('tsh install');
  expect(autocompleteCommandNames).toContain('tsh uninstall');
});

it('does not return tsh install & uninstall autocomplete command on Linux', () => {
  const appContext = new MockAppContext({ platform: 'linux' });
  const commandLauncher = new CommandLauncher(appContext);
  const autocompleteCommandNames = commandLauncher
    .getAutocompleteCommands()
    .map(c => c.displayName);

  expect(autocompleteCommandNames).not.toContain('tsh install');
  expect(autocompleteCommandNames).not.toContain('tsh uninstall');
});

it('does not return tsh install & uninstall autocomplete command on Windows', () => {
  const appContext = new MockAppContext({ platform: 'win32' });
  const commandLauncher = new CommandLauncher(appContext);
  const autocompleteCommandNames = commandLauncher
    .getAutocompleteCommands()
    .map(c => c.displayName);

  expect(autocompleteCommandNames).not.toContain('tsh install');
  expect(autocompleteCommandNames).not.toContain('tsh uninstall');
});
