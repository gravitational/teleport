import { QuickInputService } from './quickInputService';

import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import * as pickers from './quickPickers';
import {
  AutocompletePartialMatch,
  SuggestionCmd,
  SuggestionSshLogin,
} from './types';

afterEach(() => {
  jest.restoreAllMocks();
});

jest.mock('teleterm/ui/commandLauncher');
jest.mock('teleterm/ui/services/clusters');
jest.mock('teleterm/ui/services/workspacesService');

const CommandLauncherMock = CommandLauncher as jest.MockedClass<
  typeof CommandLauncher
>;
const ClustersServiceMock = ClustersService as jest.MockedClass<
  typeof ClustersService
>;
const WorkspacesServiceMock = WorkspacesService as jest.MockedClass<
  typeof WorkspacesService
>;

const onlyTshSshCommand = [
  {
    name: 'autocomplete.tsh-ssh',
    displayName: 'tsh ssh',
    description: '',
    run: () => {},
  },
];

function mockCommandLauncherAutocompleteCommands(
  commandLauncherMock: jest.MockedClass<typeof CommandLauncher>,
  commands: {
    name: string;
    displayName: string;
    description: string;
    run: () => void;
  }[]
) {
  jest
    .spyOn(commandLauncherMock.prototype, 'getAutocompleteCommands')
    .mockImplementation(() => {
      return commands;
    });
}

test('getAutocompleteResult returns correct result for a command suggestion with empty input', () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult = quickInputService.getAutocompleteResult('');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: '',
    startIndex: 0,
  });
  expect(autocompleteResult.command).toEqual({ kind: 'command.unknown' });
});

test('getAutocompleteResult returns correct result for a command suggestion', () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult = quickInputService.getAutocompleteResult('ts');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: 'ts',
    startIndex: 0,
  });
  expect(autocompleteResult.command).toEqual({ kind: 'command.unknown' });
});

test('getAutocompleteResult returns correct result for an SSH login suggestion', () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(pickers.QuickSshLoginPicker.prototype, 'getAutocompleteResult')
    .mockImplementation((input: string, startIndex: number) => {
      return {
        kind: 'autocomplete.partial-match',
        suggestions: [
          {
            kind: 'suggestion.ssh-login',
            token: 'root',
            appendToToken: '@',
            data: 'root',
          },
        ],
        targetToken: { startIndex, value: input },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult =
    quickInputService.getAutocompleteResult('tsh ssh roo');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: 'roo',
    startIndex: 8,
  });
  expect(autocompleteResult.command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'roo',
  });
});

test('getAutocompleteResult returns correct result for an SSH login suggestion with spaces between arguments', () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(pickers.QuickSshLoginPicker.prototype, 'getAutocompleteResult')
    .mockImplementation((input: string, startIndex: number) => {
      return {
        kind: 'autocomplete.partial-match',
        suggestions: [
          {
            kind: 'suggestion.ssh-login',
            token: 'barfoo',
            appendToToken: '@',
            data: 'barfoo',
          },
        ],
        targetToken: { startIndex, value: input },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult =
    quickInputService.getAutocompleteResult('   tsh ssh    bar');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: 'bar',
    startIndex: 14,
  });
  expect(autocompleteResult.command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'bar',
  });
});

test('getAutocompleteResult returns correct result for a database name suggestion', () => {
  mockCommandLauncherAutocompleteCommands(CommandLauncherMock, [
    {
      name: 'autocomplete.tsh-proxy-db',
      displayName: 'tsh proxy db',
      description: '',
      run: () => {},
    },
  ]);
  jest
    .spyOn(WorkspacesServiceMock.prototype, 'getActiveWorkspace')
    .mockImplementation(() => ({
      localClusterUri: 'test_uri',
      documents: [],
      location: '',
    }));
  jest
    .spyOn(ClustersServiceMock.prototype, 'searchDbs')
    .mockImplementation(() => {
      return [
        {
          hostname: 'foobar',
          uri: '',
          name: '',
          desc: '',
          protocol: '',
          type: '',
          addr: '',
          labelsList: null,
        },
      ];
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult =
    quickInputService.getAutocompleteResult('tsh proxy db foo');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: 'foo',
    startIndex: 13,
  });
  expect(autocompleteResult.command).toEqual({ kind: 'command.unknown' });
});

test("getAutocompleteResult doesn't return any suggestions if the only suggestion completely matches the target token", () => {
  jest.mock('./quickPickers');
  const QuickCommandPickerMock = pickers.QuickCommandPicker as jest.MockedClass<
    typeof pickers.QuickCommandPicker
  >;
  jest
    .spyOn(QuickCommandPickerMock.prototype, 'getAutocompleteResult')
    .mockImplementation(() => {
      return {
        kind: 'autocomplete.partial-match',
        suggestions: [
          {
            kind: 'suggestion.ssh-login',
            token: 'foobar',
            appendToToken: '@',
            data: 'foobar',
          },
        ],
        targetToken: {
          startIndex: 0,
          value: 'foobar',
        },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult = quickInputService.getAutocompleteResult('foobar');
  expect(autocompleteResult.kind).toBe('autocomplete.no-match');
  expect(autocompleteResult.command).toEqual({ kind: 'command.unknown' });
});

test('getAutocompleteResult returns no match if any of the pickers returns partial match with empty array', () => {
  jest.mock('./quickPickers');
  const QuickCommandPickerMock = pickers.QuickCommandPicker as jest.MockedClass<
    typeof pickers.QuickCommandPicker
  >;
  jest
    .spyOn(QuickCommandPickerMock.prototype, 'getAutocompleteResult')
    .mockImplementation(() => {
      return {
        kind: 'autocomplete.partial-match',
        suggestions: [],
        targetToken: {
          startIndex: 0,
          value: '',
        },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult = quickInputService.getAutocompleteResult('');
  expect(autocompleteResult.kind).toBe('autocomplete.no-match');
  expect(autocompleteResult.command).toEqual({ kind: 'command.unknown' });
});

test("the SSH login autocomplete isn't shown if there's no space after `tsh ssh`", () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult = quickInputService.getAutocompleteResult('tsh ssh');
  expect(autocompleteResult.kind).toBe('autocomplete.no-match');
  expect(autocompleteResult.command).toEqual({ kind: 'command.unknown' });
});

test("the SSH login autocomplete is shown only if there's at least one space after `tsh ssh`", () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(pickers.QuickSshLoginPicker.prototype, 'getAutocompleteResult')
    .mockImplementation((input: string, startIndex: number) => {
      return {
        kind: 'autocomplete.partial-match',
        suggestions: [
          {
            kind: 'suggestion.ssh-login',
            token: 'barfoo',
            appendToToken: '@',
            data: 'barfoo',
          },
        ],
        targetToken: { startIndex, value: input },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult =
    quickInputService.getAutocompleteResult('tsh ssh ');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: '',
    startIndex: 8,
  });
  expect(autocompleteResult.command).toEqual({ kind: 'command.unknown' });
});

test('getAutocompleteResult returns correct result for an SSH host suggestion right after user@', () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(WorkspacesServiceMock.prototype, 'getActiveWorkspace')
    .mockImplementation(() => ({
      localClusterUri: 'test_uri',
      documents: [],
      location: '',
    }));
  jest
    .spyOn(ClustersServiceMock.prototype, 'searchServers')
    .mockImplementation(() => {
      return [
        {
          hostname: 'bazbar',
          name: '',
          addr: '',
          uri: '',
          tunnel: false,
          labelsList: null,
        },
      ];
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult =
    quickInputService.getAutocompleteResult('tsh ssh user@');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: '',
    startIndex: 13,
  });
  expect(autocompleteResult.command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'user@',
  });
});

test('getAutocompleteResult returns correct result for a partial match on an SSH host suggestion', () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(ClustersServiceMock.prototype, 'searchServers')
    .mockImplementation(() => {
      return [
        {
          hostname: 'bazbar',
          name: '',
          addr: '',
          uri: '',
          tunnel: false,
          labelsList: null,
        },
      ];
    });
  jest
    .spyOn(WorkspacesServiceMock.prototype, 'getActiveWorkspace')
    .mockImplementation(() => ({
      localClusterUri: 'test_uri',
      documents: [],
      location: '',
    }));
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult = quickInputService.getAutocompleteResult(
    '   tsh ssh    foo@baz'
  );
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: 'baz',
    startIndex: 18,
  });
  expect(autocompleteResult.command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'foo@baz',
  });
});

test("getAutocompleteResult returns the first argument as loginHost when there's no @ sign", () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(pickers.QuickSshLoginPicker.prototype, 'getAutocompleteResult')
    .mockImplementation((input: string, startIndex: number) => {
      return {
        kind: 'autocomplete.partial-match',
        suggestions: [
          {
            kind: 'suggestion.ssh-login',
            token: 'barfoo',
            appendToToken: '@',
            data: 'barfoo',
          },
        ],
        targetToken: { startIndex, value: input },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );

  const autocompleteResult =
    quickInputService.getAutocompleteResult('tsh ssh bar');
  expect(autocompleteResult.kind).toBe('autocomplete.partial-match');
  expect((autocompleteResult as AutocompletePartialMatch).targetToken).toEqual({
    value: 'bar',
    startIndex: 8,
  });
  expect(autocompleteResult.command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'bar',
  });
});

test('picking a command suggestion in an empty input autocompletes the command', () => {
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );
  quickInputService.setState({ inputValue: '' });

  const targetToken = {
    startIndex: 0,
    value: '',
  };
  const cmd: SuggestionCmd = {
    kind: 'suggestion.cmd',
    token: 'tsh ssh',
    data: {
      name: 'autocomplete.tsh-ssh',
      displayName: 'tsh ssh',
      description: '',
    },
  };
  quickInputService.pickSuggestion(targetToken, cmd);

  expect(quickInputService.getInputValue()).toBe('tsh ssh');
});

test('picking a command suggestion in an input with a single space preserves the space', () => {
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );
  quickInputService.setState({ inputValue: ' ' });

  const targetToken = {
    startIndex: 1,
    value: '',
  };
  const cmd: SuggestionCmd = {
    kind: 'suggestion.cmd',
    token: 'tsh ssh',
    data: {
      name: 'autocomplete.tsh-ssh',
      displayName: 'tsh ssh',
      description: '',
    },
  };
  quickInputService.pickSuggestion(targetToken, cmd);

  expect(quickInputService.getInputValue()).toBe(' tsh ssh');
});

test('picking an SSH login suggestion replaces target token in input value', () => {
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );
  quickInputService.setState({ inputValue: 'tsh ssh roo --foo' });

  const targetToken = {
    value: 'roo',
    startIndex: 8,
  };
  const sshLogin: SuggestionSshLogin = {
    kind: 'suggestion.ssh-login',
    token: 'root',
    appendToToken: '@',
    data: 'root',
  };
  quickInputService.pickSuggestion(targetToken, sshLogin);

  expect(quickInputService.getInputValue()).toBe('tsh ssh root@ --foo');
});

test('pickSuggestion appends the appendToToken field to the token', () => {
  const quickInputService = new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined)
  );
  quickInputService.setState({ inputValue: 'tsh ssh foo' });

  const targetToken = {
    value: 'foo',
    startIndex: 8,
  };
  const sshLogin: SuggestionSshLogin = {
    kind: 'suggestion.ssh-login',
    token: 'foobar',
    appendToToken: '@barbaz',
    data: 'foobar',
  };
  quickInputService.pickSuggestion(targetToken, sshLogin);

  expect(quickInputService.getInputValue()).toBe('tsh ssh foobar@barbaz');
});
