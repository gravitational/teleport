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

import { Store, useStore } from 'shared/libs/stores';
import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import * as pickers from './quickPickers';
import { AutocompleteResult, AutocompleteToken, Suggestion } from './types';

type State = {
  inputValue: string;
  visible: boolean;
};

export class QuickInputService extends Store<State> {
  private quickCommandPicker: pickers.QuickCommandPicker;
  lastFocused: WeakRef<HTMLElement>;

  constructor(
    launcher: CommandLauncher,
    clustersService: ClustersService,
    workspacesService: WorkspacesService
  ) {
    super();
    this.lastFocused = new WeakRef(document.createElement('div'));
    this.quickCommandPicker = new pickers.QuickCommandPicker(this, launcher);
    this.setState({
      inputValue: '',
    });

    const sshLoginPicker = new pickers.QuickSshLoginPicker(
      workspacesService,
      clustersService
    );
    const serverPicker = new pickers.QuickServerPicker(
      workspacesService,
      clustersService
    );
    const databasePicker = new pickers.QuickDatabasePicker(
      workspacesService,
      clustersService
    );

    this.quickCommandPicker.registerPickerForCommand(
      'tsh ssh',
      new pickers.QuickTshSshPicker(sshLoginPicker, serverPicker)
    );
    this.quickCommandPicker.registerPickerForCommand(
      'tsh proxy db',
      new pickers.QuickTshProxyDbPicker(databasePicker)
    );
  }

  state: State = {
    inputValue: '',
    visible: false,
  };

  // TODO: There's no "back" in the new command bar. We can probably just remove this method and the
  // behavior related to it?
  goBack = () => {
    this.setState({
      inputValue: '',
      visible: false,
    });

    const el = this.lastFocused.deref();
    el?.focus();
  };

  show = () => {
    this.setState({
      visible: true,
    });
  };

  hide = () => {
    this.setState({
      visible: false,
    });
  };

  // Parses the input string and returns AutocompleteResult which includes information about which
  // token we currently show the autocomplete for and what are the autocomplete suggestions (items)
  // to show.
  //
  // TODO(ravicious): This function needs to take cursor index into account instead of assuming that
  // you want to complete only what's at the end of the input string.
  getAutocompleteResult(input: string): AutocompleteResult {
    const autocompleteResult =
      this.quickCommandPicker.getAutocompleteResult(input);

    // Automatically handle some universal edge cases so that each individual picker doesn't have to
    // care about them.
    if (autocompleteResult.kind === 'autocomplete.partial-match') {
      const { targetToken, suggestions } = autocompleteResult;
      const isEmpty = suggestions.length === 0;
      // Don't show suggestions if the only suggestion completely matches the target token.
      const hasSingleCompleteMatch =
        suggestions.length === 1 && suggestions[0].token === targetToken.value;

      if (isEmpty || hasSingleCompleteMatch) {
        return {
          kind: 'autocomplete.no-match',
          command: autocompleteResult.command,
        };
      }
    }

    return autocompleteResult;
  }

  // Replaces the token that is being autocompleted with the token from the suggestion.
  // `tsh ssh roo` becomes `tsh ssh root`
  //
  // However, we also preserve anything after the token so that in the future we might effortlessly
  // support cursor index. So `tsh ssh roo --cluster=bar` becomes `tsh ssh root --cluster=bar`.
  pickSuggestion(targetToken: AutocompleteToken, suggestion: Suggestion) {
    const { inputValue } = this.state;
    const insertedToken = suggestion.token + (suggestion.appendToToken || '');
    const newInputValue =
      inputValue.substring(0, targetToken.startIndex) +
      insertedToken +
      inputValue.substring(targetToken.startIndex + targetToken.value.length);

    // Keep the autocomplete visible if something was appended to the token. If nothing was appended
    // to the token then we know that we don't have any further suggestions to show.
    //
    // Consider these situations:
    //
    // 1. You type "tsh s" and choose "tsh ssh" from suggestions. The input becomes "tsh ssh " and
    // you see the autocomplete for the SSH login.
    //
    // 2. You type "tsh ssh roo" and choose "root" from suggestions. The input becomes "tsh ssh
    // root@" and you see the autocomplete for the SSH host.
    //
    // 3. You type "tsh ssh root@foo" and choose "foobar" from suggestions. The input becomes "tsh
    // ssh root@foobar". You don't see any further suggestions.
    //
    // In situation 3, it's crucial that we hide the autocomplete, as there might be other servers
    // that match "foobar", but the user already made a conscious choice of picking a specific
    // server.
    const shouldRemainVisible = !!suggestion.appendToToken;

    this.setState({
      inputValue: newInputValue,
      visible: shouldRemainVisible,
    });
  }

  getInputValue = () => {
    return this.state.inputValue;
  };

  setInputValue = (value: string) => {
    this.setState({
      inputValue: value,
      // Changing the input through the UI should always make the autocomplete box visible in case
      // there are any suggestions.
      visible: true,
    });
  };

  clearInputValueAndHide = () => {
    this.setState({
      inputValue: '',
      visible: false,
    });
  };

  useState() {
    return useStore<QuickInputService>(this).state;
  }
}
