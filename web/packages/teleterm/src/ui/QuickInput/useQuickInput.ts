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

import React, { useEffect } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';

export default function useQuickInput() {
  const { quickInputService: serviceQuickInput } = useAppContext();
  const { visible, inputValue } = serviceQuickInput.useState();
  const [activeItem, setActiveItem] = React.useState(0);
  const autocompleteResult = React.useMemo(
    () => serviceQuickInput.getAutocompleteResult(inputValue),
    [inputValue]
  );
  const hasListItems = autocompleteResult.kind === 'autocomplete.partial-match';
  const picker = autocompleteResult.picker;

  const onFocus = (e: any) => {
    if (e.relatedTarget) {
      serviceQuickInput.lastFocused = new WeakRef(e.relatedTarget);
    }
  };

  const onActiveItem = (index: number) => {
    if (!hasListItems) {
      return;
    }
    setActiveItem(index);
  };

  const onPickItem = (index: number) => {
    if (!hasListItems) {
      return;
    }
    setActiveItem(index);
    picker.onPick(autocompleteResult.listItems[index]);
  };

  const onBack = () => {
    serviceQuickInput.goBack();
    setActiveItem(0);
  };

  useKeyboardShortcuts({
    'focus-global-search': () => {
      serviceQuickInput.show();
    },
  });

  useEffect(() => {
    setActiveItem(0);
  }, [picker]);

  return {
    visible,
    autocompleteResult,
    activeItem,
    inputValue,
    onFocus,
    onBack,
    onPickItem,
    onActiveItem,
    onInputChange: serviceQuickInput.setInputValue,
    onHide: serviceQuickInput.hide,
    onShow: serviceQuickInput.show,
  };
}

export type State = ReturnType<typeof useQuickInput>;
