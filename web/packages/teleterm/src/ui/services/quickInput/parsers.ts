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

import { CommandLauncher } from 'teleterm/ui/commandLauncher';

import {
  AutocompleteCommand,
  AutocompleteToken,
  SuggestionCmd,
  QuickInputParser,
  ParseResult,
} from './types';
import * as suggesters from './suggesters';

// Pair of helper values to return when a given parser arrives at a situation where it knows there
// will be no suggestions to return (and hence also no useful target token to return).
const emptyTargetToken: AutocompleteToken = { value: '', startIndex: 0 };
const noSuggestions = () => Promise.resolve([]);

export class QuickCommandParser implements QuickInputParser {
  private parserRegistry: Map<string, QuickInputParser>;

  constructor(private launcher: CommandLauncher) {
    this.parserRegistry = new Map();
  }

  registerParserForCommand(command: string, parser: QuickInputParser) {
    this.parserRegistry.set(command, parser);
  }

  // TODO(ravicious): Handle env vars.
  parse(rawInput: string): ParseResult {
    // We can safely ignore any whitespace at the start. However, `startIndex` needs to account for
    // any removed whitespace.
    const input = rawInput.trimStart();
    const targetToken = {
      value: input,
      startIndex: rawInput.indexOf(input),
    };

    // Return all commands if there's no input.
    if (input === '') {
      const autocompleteCommands = this.launcher.getAutocompleteCommands();

      return {
        targetToken,
        command: { kind: 'command.unknown' },
        getSuggestions: () =>
          Promise.resolve(
            this.mapAutocompleteCommandsToSuggestions(autocompleteCommands)
          ),
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
        targetToken: emptyTargetToken,
        command: { kind: 'command.unknown' },
        getSuggestions: noSuggestions,
      };
    }

    if (matchingAutocompleteCommands.length > 1) {
      return {
        targetToken,
        command: { kind: 'command.unknown' },
        getSuggestions: () =>
          Promise.resolve(
            this.mapAutocompleteCommandsToSuggestions(
              matchingAutocompleteCommands
            )
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
      // so that the next parser has the correct index for the target token.
      const commandStartIndex = targetToken.startIndex + commandToken.length;
      const nextQuickInputParser = this.parserRegistry.get(commandToken);

      return nextQuickInputParser.parse(
        inputWithoutCommandPrefix,
        commandStartIndex
      );
    }

    // Handles a non-complete match with only a single matching command, for example if the input is
    // `tsh ss` (`tsh ssh` should be the only matching command then).
    return {
      targetToken,
      command: { kind: 'command.unknown' },
      getSuggestions: () =>
        Promise.resolve(
          this.mapAutocompleteCommandsToSuggestions(
            matchingAutocompleteCommands
          )
        ),
    };
  }

  private mapAutocompleteCommandsToSuggestions(
    commands: { displayName: string; description: string }[]
  ): SuggestionCmd[] {
    return commands.map(cmd => {
      const acceptsNoArguments =
        this.parserRegistry.get(cmd.displayName) instanceof
        QuickNoArgumentsParser;
      // If a command accepts arguments, let's append a space when that suggestion is picked.
      // This allows us to immediately show autocomplete for the first argument of the command.
      const appendToToken = acceptsNoArguments ? '' : ' ';

      return {
        kind: 'suggestion.cmd' as const,
        token: cmd.displayName,
        appendToToken: appendToToken,
        data: cmd,
      };
    });
  }
}

export class QuickTshSshParser implements QuickInputParser {
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
    private sshLoginSuggester: suggesters.QuickSshLoginSuggester,
    private serverSuggester: suggesters.QuickServerSuggester
  ) {}

  // TODO: Support cluster arg.
  parse(rawInput: string, startIndex: number): ParseResult {
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
      const targetToken = { value: '', startIndex };
      return {
        targetToken,
        command,
        getSuggestions: () =>
          this.sshLoginSuggester.getSuggestions(targetToken.value),
      };
    }

    const hostMatch = input.match(this.totalSshLoginAndHostRegex);

    if (hostMatch) {
      const command = {
        kind: 'command.tsh-ssh' as const,
        loginHost: hostMatch[0],
      };
      const hostStartIndex = startIndex + hostMatch.groups.loginPart.length;
      const targetToken = {
        value: hostMatch.groups.hostPart,
        startIndex: hostStartIndex,
      };

      return {
        targetToken,
        command,
        getSuggestions: () =>
          this.serverSuggester.getSuggestions(targetToken.value),
      };
    }

    const loginMatch = input.match(this.totalSshLoginRegex);

    if (loginMatch) {
      const command = {
        kind: 'command.tsh-ssh' as const,
        loginHost: loginMatch[0],
      };
      const targetToken = {
        value: loginMatch[0],
        startIndex,
      };
      return {
        targetToken,
        command,
        getSuggestions: () =>
          this.sshLoginSuggester.getSuggestions(targetToken.value),
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
      command: { kind: 'command.unknown' },
      targetToken: emptyTargetToken,
      getSuggestions: noSuggestions,
    };
  }
}

export class QuickTshProxyDbParser implements QuickInputParser {
  private totalDbNameRegex = /^\S+$/i;

  constructor(private databaseSuggester: suggesters.QuickDatabaseSuggester) {}

  parse(rawInput: string, startIndex: number): ParseResult {
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

    // Show autocomplete only after at least one space after `tsh proxy db`.
    if (rawInput !== '' && input === '') {
      const targetToken = {
        value: '',
        startIndex,
      };
      return {
        targetToken,
        command: { kind: 'command.unknown' },
        getSuggestions: () =>
          this.databaseSuggester.getSuggestions(targetToken.value),
      };
    }

    const dbNameMatch = input.match(this.totalDbNameRegex);

    if (dbNameMatch) {
      const targetToken = {
        value: dbNameMatch[0],
        startIndex,
      };
      return {
        targetToken,
        command: { kind: 'command.unknown' },
        getSuggestions: () =>
          this.databaseSuggester.getSuggestions(targetToken.value),
      };
    }

    return {
      targetToken: emptyTargetToken,
      command: { kind: 'command.unknown' },
      getSuggestions: noSuggestions,
    };
  }
}

// QuickNoArgumentsParser is useful in situations where a command does not accept any arguments.
// If QuickNoArgumentsParser is registered as the parser for that command, selecting that command
// from suggestions will simply close autocomplete. Pressing Enter again will execute the command
// passed to the constructor of QuickNoArgumentsParser.
export class QuickNoArgumentsParser implements QuickInputParser {
  constructor(private command: AutocompleteCommand) {}

  parse(rawInput: string, startIndex: number): ParseResult {
    const targetToken = {
      value: '',
      startIndex,
    };

    return {
      targetToken,
      command: this.command,
      getSuggestions: noSuggestions,
    };
  }
}
