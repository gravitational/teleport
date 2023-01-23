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
