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
  const [activeSuggestion, setActiveSuggestion] = React.useState(0);
  const autocompleteResult = React.useMemo(
    () => serviceQuickInput.getAutocompleteResult(inputValue),
    [inputValue]
  );
  const hasSuggestions =
    autocompleteResult.kind === 'autocomplete.partial-match';

  const onFocus = (e: any) => {
    if (e.relatedTarget) {
      serviceQuickInput.lastFocused = new WeakRef(e.relatedTarget);
    }
  };

  const onActiveSuggestion = (index: number) => {
    if (!hasSuggestions) {
      return;
    }
    setActiveSuggestion(index);
  };

  const onPickSuggestion = (index: number) => {
    if (!hasSuggestions) {
      return;
    }

    const suggestion = autocompleteResult.suggestions[index];

    setActiveSuggestion(index);
    serviceQuickInput.pickSuggestion(
      autocompleteResult.targetToken,
      suggestion
    );
  };

  const onBack = () => {
    serviceQuickInput.goBack();
    setActiveSuggestion(0);
  };

  useKeyboardShortcuts({
    'focus-global-search': () => {
      serviceQuickInput.show();
    },
  });

  // Reset active suggestion when the suggestion list changes.
  // We extract just the tokens and stringify the list to avoid stringifying big objects.
  // See https://github.com/facebook/react/issues/14476#issuecomment-471199055
  useEffect(() => {
    setActiveSuggestion(0);
  }, [
    hasSuggestions &&
      JSON.stringify(
        autocompleteResult.suggestions.map(suggestion => suggestion.token)
      ),
  ]);

  return {
    visible,
    autocompleteResult,
    activeSuggestion,
    inputValue,
    onFocus,
    onBack,
    onPickSuggestion,
    onActiveSuggestion,
    onInputChange: serviceQuickInput.setInputValue,
    onHide: serviceQuickInput.hide,
    onShow: serviceQuickInput.show,
  };
}

export type State = ReturnType<typeof useQuickInput>;
