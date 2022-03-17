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
import {
  AutocompleteResult,
  AutocompletePartialMatch,
} from 'teleterm/ui/services/quickInput/types';

export default function useQuickInput() {
  const {
    quickInputService,
    workspacesService,
    clustersService,
    commandLauncher,
  } = useAppContext();
  workspacesService.useState();
  const documentsService =
    workspacesService.getActiveWorkspaceDocumentService();
  const { visible, inputValue } = quickInputService.useState();
  const [activeSuggestion, setActiveSuggestion] = React.useState(0);
  const autocompleteResult = React.useMemo(
    () => quickInputService.getAutocompleteResult(inputValue),
    [inputValue]
  );
  const hasSuggestions =
    autocompleteResult.kind === 'autocomplete.partial-match';

  const onFocus = (e: any) => {
    if (e.relatedTarget) {
      quickInputService.lastFocused = new WeakRef(e.relatedTarget);
    }
  };

  const onActiveSuggestion = (index: number) => {
    if (!hasSuggestions) {
      return;
    }
    setActiveSuggestion(index);
  };

  const onEnter = (index?: number) => {
    if (!hasSuggestions || !visible) {
      executeCommand(autocompleteResult);
      return;
    }

    // Passing `autocompleteResult` directly to narrow down AutocompleteResult type to
    // AutocompletePartialMatch.
    pickSuggestion(autocompleteResult, index);
  };

  const executeCommand = (autocompleteResult: AutocompleteResult) => {
    const { command } = autocompleteResult;

    switch (command.kind) {
      case 'command.unknown': {
        documentsService.openNewTerminal(inputValue);
        break;
      }
      case 'command.tsh-ssh': {
        const { localClusterUri } = workspacesService.getActiveWorkspace();

        commandLauncher.executeCommand('tsh-ssh', {
          loginHost: command.loginHost,
          localClusterUri,
        });
        break;
      }
    }

    quickInputService.clearInputValueAndHide();
  };

  const pickSuggestion = (
    autocompleteResult: AutocompletePartialMatch,
    index?: number
  ) => {
    const suggestion = autocompleteResult.suggestions[index];

    setActiveSuggestion(index);
    quickInputService.pickSuggestion(
      autocompleteResult.targetToken,
      suggestion
    );
  };

  const onBack = () => {
    setActiveSuggestion(0);

    // If there are suggestions to show, the first onBack call should always just close the
    // suggestions and the second call should actually go back.
    if (visible && hasSuggestions) {
      quickInputService.hide();
    } else {
      quickInputService.goBack();
    }
  };

  useKeyboardShortcuts({
    'focus-global-search': () => {
      quickInputService.show();
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
    onEnter,
    onActiveSuggestion,
    onInputChange: quickInputService.setInputValue,
    onHide: quickInputService.hide,
    onShow: quickInputService.show,
  };
}

export type State = ReturnType<typeof useQuickInput>;
