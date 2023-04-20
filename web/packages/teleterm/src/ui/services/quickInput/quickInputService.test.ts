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

import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import { ResourcesService } from 'teleterm/ui/services/resources';

import { getEmptyPendingAccessRequest } from '../workspacesService/accessRequestsService';

import { QuickInputService } from './quickInputService';
import * as parsers from './parsers';
import * as suggestors from './suggesters';
import { SuggestionCmd, SuggestionSshLogin } from './types';

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
const ResourcesServiceMock = ResourcesService as jest.MockedClass<
  typeof ResourcesService
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

function createQuickInputService() {
  return new QuickInputService(
    new CommandLauncherMock(undefined),
    new ClustersServiceMock(undefined, undefined, undefined, undefined),
    new ResourcesServiceMock(undefined),
    new WorkspacesServiceMock(undefined, undefined, undefined, undefined)
  );
}

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

test('parse returns correct result for a command suggestion with empty input', async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } = quickInputService.parse('');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: '',
    startIndex: 0,
  });
  expect(command).toEqual({ kind: 'command.unknown' });
});

test('parse returns correct result for a command suggestion', async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } =
    quickInputService.parse('ts');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: 'ts',
    startIndex: 0,
  });
  expect(command).toEqual({ kind: 'command.unknown' });
});

test('parse returns correct result for an SSH login suggestion', async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(suggestors.QuickSshLoginSuggester.prototype, 'getSuggestions')
    .mockImplementation(async () => {
      return [
        {
          kind: 'suggestion.ssh-login',
          token: 'root',
          appendToToken: '@',
          data: 'root',
        },
      ];
    });
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } =
    quickInputService.parse('tsh ssh roo');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: 'roo',
    startIndex: 8,
  });
  expect(command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'roo',
  });
});

test('parse returns correct result for an SSH login suggestion with spaces between arguments', async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(suggestors.QuickSshLoginSuggester.prototype, 'getSuggestions')
    .mockImplementation(async () => {
      return [
        {
          kind: 'suggestion.ssh-login',
          token: 'barfoo',
          appendToToken: '@',
          data: 'barfoo',
        },
      ];
    });
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } =
    quickInputService.parse('   tsh ssh    bar');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: 'bar',
    startIndex: 14,
  });
  expect(command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'bar',
  });
});

test('parse returns correct result for a database name suggestion', async () => {
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
      accessRequests: {
        assumed: {},
        isBarCollapsed: false,
        pending: getEmptyPendingAccessRequest(),
      },
      localClusterUri: '/clusters/test_uri',
      documents: [],
      location: '/docs/1',
    }));
  jest
    .spyOn(ResourcesServiceMock.prototype, 'fetchDatabases')
    .mockImplementation(() => {
      return Promise.resolve({
        agentsList: [
          {
            hostname: 'foobar',
            uri: '/clusters/test_uri/dbs/foobar',
            name: '',
            desc: '',
            protocol: '',
            type: '',
            addr: '',
            labelsList: null,
          },
        ],
        startKey: '',
        totalCount: 1,
      });
    });
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } =
    quickInputService.parse('tsh proxy db foo');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: 'foo',
    startIndex: 13,
  });
  expect(command).toEqual({ kind: 'command.unknown' });
});

test("parse doesn't return any suggestions if the only suggestion completely matches the target token", async () => {
  jest.mock('./parsers');
  const QuickCommandParserMock = parsers.QuickCommandParser as jest.MockedClass<
    typeof parsers.QuickCommandParser
  >;
  jest
    .spyOn(QuickCommandParserMock.prototype, 'parse')
    .mockImplementation(() => {
      return {
        getSuggestions: () =>
          Promise.resolve([
            {
              kind: 'suggestion.ssh-login',
              token: 'foobar',
              appendToToken: '@',
              data: 'foobar',
            },
          ]),
        targetToken: {
          startIndex: 0,
          value: 'foobar',
        },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = createQuickInputService();

  const { getSuggestions, command } = quickInputService.parse('foobar');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(0);
  expect(command).toEqual({ kind: 'command.unknown' });
});

test('parse returns no suggestions if any of the parsers returns empty suggestions array', async () => {
  jest.mock('./parsers');
  const QuickCommandParserMock = parsers.QuickCommandParser as jest.MockedClass<
    typeof parsers.QuickCommandParser
  >;
  jest
    .spyOn(QuickCommandParserMock.prototype, 'parse')
    .mockImplementation(() => {
      return {
        getSuggestions: () => Promise.resolve([]),
        targetToken: {
          startIndex: 0,
          value: '',
        },
        command: { kind: 'command.unknown' },
      };
    });
  const quickInputService = createQuickInputService();

  const { command, getSuggestions } = quickInputService.parse('');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(0);
  expect(command).toEqual({ kind: 'command.unknown' });
});

test("the SSH login autocomplete isn't shown if there's no space after `tsh ssh`", async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  const quickInputService = createQuickInputService();

  const { command, getSuggestions } = quickInputService.parse('tsh ssh');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(0);
  expect(command).toEqual({ kind: 'command.unknown' });
});

test("the SSH login autocomplete is shown only if there's at least one space after `tsh ssh`", async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(suggestors.QuickSshLoginSuggester.prototype, 'getSuggestions')
    .mockImplementation(async () => {
      return [
        {
          kind: 'suggestion.ssh-login',
          token: 'barfoo',
          appendToToken: '@',
          data: 'barfoo',
        },
      ];
    });
  const quickInputService = createQuickInputService();

  const { command, targetToken, getSuggestions } =
    quickInputService.parse('tsh ssh ');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: '',
    startIndex: 8,
  });
  expect(command).toEqual({ kind: 'command.unknown' });
});

test('parse returns correct result for an SSH host suggestion right after user@', async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(ResourcesServiceMock.prototype, 'fetchServers')
    .mockImplementation(() => {
      return Promise.resolve({
        agentsList: [
          {
            hostname: 'bazbar',
            name: '',
            addr: '',
            uri: '/clusters/foo/servers/bazbar',
            tunnel: false,
            labelsList: null,
          },
        ],
        startKey: '',
        totalCount: 1,
      });
    });
  jest
    .spyOn(WorkspacesServiceMock.prototype, 'getActiveWorkspace')
    .mockImplementation(() => ({
      accessRequests: {
        assumed: {},
        isBarCollapsed: false,
        pending: getEmptyPendingAccessRequest(),
      },
      localClusterUri: '/clusters/test_uri',
      documents: [],
      location: '/docs/1',
    }));
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } =
    quickInputService.parse('tsh ssh user@');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: '',
    startIndex: 13,
  });
  expect(command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'user@',
  });
});

test('parse returns correct result for a partial match on an SSH host suggestion', async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(ResourcesServiceMock.prototype, 'fetchServers')
    .mockImplementation(() => {
      return Promise.resolve({
        agentsList: [
          {
            hostname: 'bazbar',
            name: '',
            addr: '',
            uri: '/clusters/foo/servers/bazbar',
            tunnel: false,
            labelsList: null,
          },
        ],
        startKey: '',
        totalCount: 1,
      });
    });
  jest
    .spyOn(WorkspacesServiceMock.prototype, 'getActiveWorkspace')
    .mockImplementation(() => ({
      accessRequests: {
        assumed: {},
        isBarCollapsed: false,
        pending: getEmptyPendingAccessRequest(),
      },
      localClusterUri: '/clusters/test_uri',
      documents: [],
      location: '/docs/1',
    }));
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } = quickInputService.parse(
    '   tsh ssh    foo@baz'
  );
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: 'baz',
    startIndex: 18,
  });
  expect(command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'foo@baz',
  });
});

test("parse returns the first argument as loginHost when there's no @ sign", async () => {
  mockCommandLauncherAutocompleteCommands(
    CommandLauncherMock,
    onlyTshSshCommand
  );
  jest
    .spyOn(suggestors.QuickSshLoginSuggester.prototype, 'getSuggestions')
    .mockImplementation(async () => {
      return [
        {
          kind: 'suggestion.ssh-login',
          token: 'barfoo',
          appendToToken: '@',
          data: 'barfoo',
        },
      ];
    });
  const quickInputService = createQuickInputService();

  const { getSuggestions, targetToken, command } =
    quickInputService.parse('tsh ssh bar');
  const suggestions = await getSuggestions();

  expect(suggestions).toHaveLength(1);
  expect(targetToken).toEqual({
    value: 'bar',
    startIndex: 8,
  });
  expect(command).toEqual({
    kind: 'command.tsh-ssh',
    loginHost: 'bar',
  });
});

test('picking a command suggestion in an empty input autocompletes the command', () => {
  const quickInputService = createQuickInputService();
  quickInputService.setState({ inputValue: '' });

  const targetToken = {
    startIndex: 0,
    value: '',
  };
  const cmd: SuggestionCmd = {
    kind: 'suggestion.cmd',
    token: 'tsh ssh',
    data: {
      displayName: 'tsh ssh',
      description: '',
    },
  };
  quickInputService.pickSuggestion(targetToken, cmd);

  expect(quickInputService.getInputValue()).toBe('tsh ssh');
});

test('picking a command suggestion in an input with a single space preserves the space', () => {
  const quickInputService = createQuickInputService();
  quickInputService.setState({ inputValue: ' ' });

  const targetToken = {
    startIndex: 1,
    value: '',
  };
  const cmd: SuggestionCmd = {
    kind: 'suggestion.cmd',
    token: 'tsh ssh',
    data: {
      displayName: 'tsh ssh',
      description: '',
    },
  };
  quickInputService.pickSuggestion(targetToken, cmd);

  expect(quickInputService.getInputValue()).toBe(' tsh ssh');
});

test('picking an SSH login suggestion replaces target token in input value', () => {
  const quickInputService = createQuickInputService();
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
  const quickInputService = createQuickInputService();
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
