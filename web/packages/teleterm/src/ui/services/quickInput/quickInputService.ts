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
import * as pickers from './quickPickers';
import { QuickInputPicker } from './types';

type State = {
  picker: QuickInputPicker;
  inputValue: string;
  visible: boolean;
};

export class QuickInputService extends Store<State> {
  quickLoginPicker: pickers.QuickLoginPicker;
  quickDbPicker: pickers.QuickDbPicker;
  quickServerPicker: pickers.QuickServerPicker;
  quickCommandPicker: pickers.QuickCommandPicker;
  lastFocused: WeakRef<HTMLElement>;

  constructor(launcher: CommandLauncher, serviceClusters: ClustersService) {
    super();
    this.lastFocused = new WeakRef(document.createElement('div'));
    this.quickDbPicker = new pickers.QuickDbPicker(launcher, serviceClusters);
    this.quickServerPicker = new pickers.QuickServerPicker(
      launcher,
      serviceClusters
    );
    this.quickLoginPicker = new pickers.QuickLoginPicker(
      launcher,
      serviceClusters
    );
    this.quickCommandPicker = new pickers.QuickCommandPicker();
    this.setState({
      picker: this.quickCommandPicker,
      inputValue: '',
    });
  }

  state: State = {
    picker: null,
    inputValue: '',
    visible: false,
  };

  goBack = () => {
    if (this.state.picker !== this.quickCommandPicker) {
      this.setState({
        picker: this.quickCommandPicker,
        inputValue: '',
      });
      return;
    }

    this.setState({
      inputValue: '',
      visible: false,
    });

    const el = this.lastFocused.deref();
    el?.focus();
  };

  show = () => {
    this.setState({
      picker: this.quickCommandPicker,
      visible: true,
    });
  };

  hide = () => {
    this.setState({
      visible: false,
    });
  };

  setInputValue = (value: string) => {
    this.setState({
      inputValue: value,
    });
  };

  useState() {
    return useStore<QuickInputService>(this).state;
  }
}
