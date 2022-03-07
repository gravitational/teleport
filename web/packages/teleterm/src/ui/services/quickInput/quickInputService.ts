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
import { AutocompleteResult } from './types';

type State = {
  inputValue: string;
  visible: boolean;
};

export class QuickInputService extends Store<State> {
  quickLoginPicker: pickers.QuickLoginPicker;
  quickDbPicker: pickers.QuickDbPicker;
  quickServerPicker: pickers.QuickServerPicker;
  quickCommandPicker: pickers.QuickCommandPicker;
  lastFocused: WeakRef<HTMLElement>;

  constructor(
    launcher: CommandLauncher,
    clustersService: ClustersService,
    workspacesService: WorkspacesService
  ) {
    super();
    this.lastFocused = new WeakRef(document.createElement('div'));
    this.quickDbPicker = new pickers.QuickDbPicker(launcher, clustersService);
    this.quickServerPicker = new pickers.QuickServerPicker(
      launcher,
      clustersService
    );
    this.quickLoginPicker = new pickers.QuickLoginPicker(
      launcher,
      clustersService
    );
    this.quickCommandPicker = new pickers.QuickCommandPicker(launcher);
    this.setState({
      inputValue: '',
    });

    const sshLoginPicker = new pickers.QuickSshLoginPicker(
      workspacesService,
      clustersService
    );

    this.quickCommandPicker.registerPickerForCommand(
      'tsh ssh',
      new pickers.QuickTshSshPicker(launcher, sshLoginPicker)
    );
    this.quickCommandPicker.registerPickerForCommand(
      'tsh proxy db',
      new pickers.QuickTshProxyDbPicker(launcher)
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

  // TODO(ravicious): This function needs to take cursor index into account instead of assuming that
  // you want to complete only what's at the end of the input string.
  getAutocompleteResult(input: string): AutocompleteResult {
    return this.quickCommandPicker.getAutocompleteResult(input);
  }

  setInputValue = (value: string) => {
    this.setState({
      inputValue: value,
    });
  };

  useState() {
    return useStore<QuickInputService>(this).state;
  }
}
