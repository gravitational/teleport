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
import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import {
  QuickInputPicker,
  SuggestionCmd,
  SuggestionSshLogin,
  AutocompleteResult,
} from './types';

export class QuickCommandPicker implements QuickInputPicker {
  private pickerRegistry: Map<string, QuickInputPicker>;

  constructor(private launcher: CommandLauncher) {
    this.pickerRegistry = new Map();
  }

  registerPickerForCommand(command: string, picker: QuickInputPicker) {
    this.pickerRegistry.set(command, picker);
  }

  onPick(suggestion: SuggestionCmd) {
    this.launcher.executeCommand(suggestion.data.name as any, null);
  }

  // TODO: Add tests.
  // `tsh ssh foo` as input should return recognized-token.
  // TODO(ravicious): Handle env vars.
  getAutocompleteResult(input: string): AutocompleteResult {
    const autocompleteCommands = this.launcher.getAutocompleteCommands();

    // We can safely ignore any whitespace at the start.
    input = input.trimStart();

    // Return all commands if there's no input.
    if (input === '') {
      return {
        kind: 'autocomplete.partial-match',
        picker: this,
        suggestions:
          this.mapAutocompleteCommandsToSuggestions(autocompleteCommands),
      };
    }

    const matchingAutocompleteCommands = this.launcher
      .getAutocompleteCommands()
      .filter(cmd => {
        const completeMatchRegex = new RegExp(`^${cmd.displayName}\\b`);
        return (
          cmd.displayName.startsWith(input) ||
          // `completeMatchRegex` handles situations where the `input` akin to "tsh ssh foo".
          // In that case, "tsh ssh" is the matching command, even though
          // `cmd.displayName` ("tsh ssh") doesn't start with `input` ("tsh ssh foo").
          completeMatchRegex.test(input)
        );
      });

    if (matchingAutocompleteCommands.length === 0) {
      return { kind: 'autocomplete.no-match', picker: this };
    }

    if (matchingAutocompleteCommands.length > 1) {
      return {
        kind: 'autocomplete.partial-match',
        picker: this,
        suggestions: this.mapAutocompleteCommandsToSuggestions(
          matchingAutocompleteCommands
        ),
      };
    }

    // The rest of the function body handles situation in which there's only one matching
    // autocomplete command.

    // Handles a complete match, for example the input is `tsh ssh`.
    const soleMatch = matchingAutocompleteCommands[0];
    const completeMatchRegex = new RegExp(`^${soleMatch.displayName}\\b`);
    const isCompleteMatch = completeMatchRegex.test(input);

    if (isCompleteMatch) {
      const valueWithoutCommandPrefix = input.replace(completeMatchRegex, '');
      const nextQuickInputPicker = this.pickerRegistry.get(
        soleMatch.displayName
      );
      return nextQuickInputPicker.getAutocompleteResult(
        valueWithoutCommandPrefix
      );
    }

    // Handles a non-complete match with only a single matching command, for example if the input is
    // `tsh ss` (`tsh ssh` should be the only matching command then).
    return {
      kind: 'autocomplete.partial-match',
      picker: this,
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
      data: cmd,
    }));
  }
}

export class QuickTshSshPicker implements QuickInputPicker {
  // TODO: Make sure this regex is okay and covers valid usernames.
  loginRegex = /^\s+([a-zA-Z0-9_-]*\b)?$/;

  constructor(
    private launcher: CommandLauncher,
    private sshLoginPicker: QuickSshLoginPicker
  ) {}

  onFilter() {
    return [];
  }

  onPick(suggestion: SuggestionCmd) {
    // TODO: Execute SSH.
    // this.launcher.executeCommand(suggestion.data.name as any, null);
  }

  // TODO: Support cluster arg.
  getAutocompleteResult(input: string): AutocompleteResult {
    const loginMatch = input.match(this.loginRegex);

    if (loginMatch) {
      return this.sshLoginPicker.getAutocompleteResult(loginMatch[1]);
    }

    return {
      kind: 'autocomplete.no-match',
      picker: this,
    };
  }
}

// TODO: Implement the rest of this class.
export class QuickTshProxyDbPicker implements QuickInputPicker {
  constructor(private launcher: CommandLauncher) {}

  onFilter() {
    return [];
  }

  onPick(suggestion: SuggestionCmd) {
    // TODO: Execute Proxy db.
    // this.launcher.executeCommand(suggestion.data.name as any, null);
  }

  getAutocompleteResult(input: string): AutocompleteResult {
    return {
      kind: 'autocomplete.no-match',
      picker: this,
    };
  }
}

export class QuickSshLoginPicker implements QuickInputPicker {
  constructor(
    private workspacesService: WorkspacesService,
    private clustersService: ClustersService
  ) {}

  filterSshLogins(input: string): SuggestionSshLogin[] {
    // TODO(ravicious): Use local cluster URI.
    // TODO(ravicious): Handle the `--cluster` tsh ssh flag.
    const rootClusterUri = this.workspacesService.getRootClusterUri();
    const cluster = this.clustersService.findCluster(rootClusterUri);
    const allLogins = cluster?.loggedInUser?.sshLoginsList || [];
    let matchingLogins: typeof allLogins;

    if (!input) {
      matchingLogins = allLogins;
    } else {
      matchingLogins = allLogins.filter(login => login.startsWith(input));
    }

    return matchingLogins.map(login => ({
      kind: 'suggestion.ssh-login' as const,
      token: login,
      data: null,
    }));
  }

  // TODO: Append the rest of the login to quickInputService's inputValue.
  onPick(suggestion: SuggestionCmd) {}

  getAutocompleteResult(input: string): AutocompleteResult {
    const suggestions = this.filterSshLogins(input);
    return {
      kind: 'autocomplete.partial-match',
      picker: this,
      suggestions,
    };
  }
}
