/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { ClustersService } from 'teleterm/ui/services/clusters';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import { QuickInputService } from 'teleterm/ui/services/quickInput';
import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import {
  QuickInputPicker,
  SuggestionCmd,
  SuggestionServer,
  SuggestionSshLogin,
  AutocompleteResult,
} from './types';

export class QuickCommandPicker implements QuickInputPicker {
  private pickerRegistry: Map<string, QuickInputPicker>;

  constructor(
    private quickInputService: QuickInputService,
    private launcher: CommandLauncher
  ) {
    this.pickerRegistry = new Map();
  }

  registerPickerForCommand(command: string, picker: QuickInputPicker) {
    this.pickerRegistry.set(command, picker);
  }

  // TODO(ravicious): Handle env vars.
  getAutocompleteResult(rawInput: string): AutocompleteResult {
    const autocompleteCommands = this.launcher.getAutocompleteCommands();
    // We can safely ignore any whitespace at the start. However, `startIndex` needs to account for
    // any removed whitespace.
    const input = rawInput.trimStart();
    const targetToken = {
      value: input,
      startIndex: rawInput.indexOf(input),
    };

    // Return all commands if there's no input.
    if (input === '') {
      return {
        kind: 'autocomplete.partial-match',
        targetToken,
        command: { kind: 'command.unknown' },
        suggestions:
          this.mapAutocompleteCommandsToSuggestions(autocompleteCommands),
      };
    }

    const matchingAutocompleteCommands = this.launcher
      .getAutocompleteCommands()
      .filter(cmd => {
        const completeMatchRegex = new RegExp(`^${cmd.displayName}\\b`, 'i');
        return (
          cmd.displayName.startsWith(input.toLowerCase()) ||
          // `completeMatchRegex` handles situations where the `input` akin to "tsh ssh foo".
          // In that case, "tsh ssh" is the matching command, even though
          // `cmd.displayName` ("tsh ssh") doesn't start with `input` ("tsh ssh foo").
          completeMatchRegex.test(input)
        );
      });

    if (matchingAutocompleteCommands.length === 0) {
      return {
        kind: 'autocomplete.no-match',
        command: { kind: 'command.unknown' },
      };
    }

    if (matchingAutocompleteCommands.length > 1) {
      return {
        kind: 'autocomplete.partial-match',
        targetToken,
        command: { kind: 'command.unknown' },
        suggestions: this.mapAutocompleteCommandsToSuggestions(
          matchingAutocompleteCommands
        ),
      };
    }

    // The rest of the function body handles situation in which there's only one matching
    // autocomplete command.

    // Handles a complete match, for example the input is `tsh ssh`.
    const soleMatch = matchingAutocompleteCommands[0];
    const commandToken = soleMatch.displayName;
    const completeMatchRegex = new RegExp(`^${commandToken}\\b`, 'i');
    const isCompleteMatch = completeMatchRegex.test(input);

    if (isCompleteMatch) {
      const inputWithoutCommandPrefix = input.replace(completeMatchRegex, '');
      // Add length of the command we just replaced with an empty string to startIndex,
      // so that the next picker has the correct index for the target token.
      const commandStartIndex = targetToken.startIndex + commandToken.length;
      const nextQuickInputPicker = this.pickerRegistry.get(commandToken);

      return nextQuickInputPicker.getAutocompleteResult(
        inputWithoutCommandPrefix,
        commandStartIndex
      );
    }

    // Handles a non-complete match with only a single matching command, for example if the input is
    // `tsh ss` (`tsh ssh` should be the only matching command then).
    return {
      kind: 'autocomplete.partial-match',
      targetToken,
      command: { kind: 'command.unknown' },
      suggestions: this.mapAutocompleteCommandsToSuggestions(
        matchingAutocompleteCommands
      ),
    };
  }

  private mapAutocompleteCommandsToSuggestions(
    commands: { name: string; displayName: string; description: string }[]
  ): SuggestionCmd[] {
    return commands.map(cmd => ({
      kind: 'suggestion.cmd' as const,
      token: cmd.displayName,
      // This allows us to immediately show autocomplete for the first argument of the command.
      appendToToken: ' ',
      data: cmd,
    }));
  }
}

export class QuickTshSshPicker implements QuickInputPicker {
  // An SSH login doesn't start with `-`, hence the special group for the first character.
  private sshLoginRegex = /[a-z0-9_][a-z0-9_-]*/i;
  private totalSshLoginRegex = new RegExp(
    `^${this.sshLoginRegex.source}$`,
    'i'
  );
  // For now we assume there's nothing else after user@host, so if we see any space after `@`, we
  // don't show any matches.
  // To support that properly, we'd need to add account for the cursor index.
  private totalSshLoginAndHostRegex = new RegExp(
    `^(?<loginPart>${this.sshLoginRegex.source}@)(?<hostPart>\\S*)$`,
    'i'
  );

  constructor(
    private sshLoginPicker: QuickSshLoginPicker,
    private serverPicker: QuickServerPicker
  ) {}

  // TODO: Support cluster arg.
  getAutocompleteResult(
    rawInput: string,
    startIndex: number
  ): AutocompleteResult {
    // We can safely ignore any whitespace at the start. However, `startIndex` needs to account for
    // any removed whitespace.
    const input = rawInput.trimStart();
    if (input === '') {
      // input is empty, so rawInput must include only whitespace.
      // Add length of the whitespace to startIndex.
      startIndex += rawInput.length;
    } else {
      startIndex += rawInput.indexOf(input);
    }

    // Show autocomplete only after at least one space after `tsh ssh`.
    if (rawInput !== '' && input === '') {
      // Returning unknown command for the same reasons as outlined at the end of this function.
      const command = {
        kind: 'command.unknown' as const,
      };
      return {
        ...this.sshLoginPicker.getAutocompleteResult('', startIndex),
        command,
      };
    }

    const hostMatch = input.match(this.totalSshLoginAndHostRegex);

    if (hostMatch) {
      const command = {
        kind: 'command.tsh-ssh' as const,
        loginHost: hostMatch[0],
      };
      const hostStartIndex = startIndex + hostMatch.groups.loginPart.length;

      return {
        ...this.serverPicker.getAutocompleteResult(
          hostMatch.groups.hostPart,
          hostStartIndex
        ),
        command,
      };
    }

    const loginMatch = input.match(this.totalSshLoginRegex);

    if (loginMatch) {
      const command = {
        kind: 'command.tsh-ssh' as const,
        loginHost: loginMatch[0],
      };
      return {
        ...this.sshLoginPicker.getAutocompleteResult(loginMatch[0], startIndex),
        command,
      };
    }

    // The code gets to this point if either `input` is empty or it has additional arguments besides
    // the first positional argument.
    //
    // In case of the input being empty, we know that at this point we don't have enough arguments
    // to successfully launch tsh ssh. But we also don't have code to handle this error case. So
    // instead we return an unknown command, so that if the user presses enter at this point, we'll
    // launch `tsh ssh` in a local shell and it'll show the appropriate error.
    //
    // In case of additional arguments, the command bar doesn't know how to handle them. This would
    // require adding a real parser which we're going to do soon. In the meantime, we just run the
    // command in a local shell to a similar effect (though the host to which someone tries to
    // connect to won't show up in the connection tracker and so on).
    return {
      kind: 'autocomplete.no-match',
      command: { kind: 'command.unknown' },
    };
  }
}

// TODO: Implement the rest of this class.
export class QuickTshProxyDbPicker implements QuickInputPicker {
  constructor() {}

  getAutocompleteResult(input: string): AutocompleteResult {
    return {
      kind: 'autocomplete.no-match',
      command: { kind: 'command.unknown' },
    };
  }
}

export class QuickSshLoginPicker implements QuickInputPicker {
  constructor(
    private workspacesService: WorkspacesService,
    private clustersService: ClustersService
  ) {}

  private filterSshLogins(input: string): SuggestionSshLogin[] {
    // TODO(ravicious): Use local cluster URI.
    // TODO(ravicious): Handle the `--cluster` tsh ssh flag.
    const rootClusterUri = this.workspacesService.getRootClusterUri();
    const cluster = this.clustersService.findCluster(rootClusterUri);
    const allLogins = cluster?.loggedInUser?.sshLoginsList || [];
    let matchingLogins: typeof allLogins;

    if (!input) {
      matchingLogins = allLogins;
    } else {
      matchingLogins = allLogins.filter(login =>
        login.startsWith(input.toLowerCase())
      );
    }

    return matchingLogins.map(login => ({
      kind: 'suggestion.ssh-login' as const,
      token: login,
      // This allows the user to immediately begin typing the hostname.
      appendToToken: '@',
      data: login,
    }));
  }

  getAutocompleteResult(input: string, startIndex: number): AutocompleteResult {
    const suggestions = this.filterSshLogins(input);
    return {
      kind: 'autocomplete.partial-match',
      suggestions,
      command: { kind: 'command.unknown' },
      targetToken: {
        startIndex,
        value: input,
      },
    };
  }
}

export class QuickServerPicker implements QuickInputPicker {
  constructor(
    private workspacesService: WorkspacesService,
    private clustersService: ClustersService
  ) {}

  private filterServers(input: string): SuggestionServer[] {
    // TODO(ravicious): Use local cluster URI.
    // TODO(ravicious): Handle the `--cluster` tsh ssh flag.
    const rootClusterUri = this.workspacesService.getRootClusterUri();
    const servers = this.clustersService.searchServers(rootClusterUri, {
      search: input,
    });

    return servers.map(server => ({
      kind: 'suggestion.server' as const,
      token: server.hostname,
      data: server,
    }));
  }

  getAutocompleteResult(input: string, startIndex: number): AutocompleteResult {
    const suggestions = this.filterServers(input);
    return {
      kind: 'autocomplete.partial-match',
      suggestions,
      command: { kind: 'command.unknown' },
      targetToken: {
        startIndex,
        value: input,
      },
    };
  }
}
