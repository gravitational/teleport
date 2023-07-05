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

import { CanceledError, useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  useKeyboardShortcuts,
  useKeyboardShortcutFormatters,
} from 'teleterm/ui/services/keyboardShortcuts';
import {
  AutocompleteCommand,
  AutocompleteToken,
  Suggestion,
} from 'teleterm/ui/services/quickInput/types';
import { routing } from 'teleterm/ui/uri';
import { KeyboardShortcutAction } from 'teleterm/services/config';

import { assertUnreachable, retryWithRelogin } from '../utils';

export default function useQuickInput() {
  const appContext = useAppContext();
  const {
    quickInputService,
    workspacesService,
    commandLauncher,
    usageService,
  } = appContext;
  workspacesService.useState();
  const documentsService =
    workspacesService.getActiveWorkspaceDocumentService();
  const { visible, inputValue } = quickInputService.useState();
  const [activeSuggestion, setActiveSuggestion] = React.useState(0);

  const parseResult = React.useMemo(
    () => quickInputService.parse(inputValue),
    // `localClusterUri` has been added to refresh suggestions from
    // `QuickSshLoginPicker` and `QuickServerPicker` when it changes
    [inputValue, workspacesService.getActiveWorkspace()?.localClusterUri]
  );

  const [suggestionsAttempt, getSuggestions] = useAsync(() =>
    retryWithRelogin(
      appContext,
      workspacesService.getActiveWorkspace()?.localClusterUri,
      () => parseResult.getSuggestions()
    )
  );

  useEffect(() => {
    async function get() {
      const [, err] = await getSuggestions();
      if (err && !(err instanceof CanceledError)) {
        appContext.notificationsService.notifyError({
          title: 'Could not fetch command bar suggestions',
          description: err.message,
        });
      }
    }

    get();
  }, [parseResult]);

  const hasSuggestions =
    suggestionsAttempt.status === 'success' &&
    suggestionsAttempt.data.length > 0;
  const openCommandBarShortcutAction: KeyboardShortcutAction = 'openCommandBar';
  const { getAccelerator } = useKeyboardShortcutFormatters();

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
      executeCommand(parseResult.command);
      return;
    }

    pickSuggestion(parseResult.targetToken, suggestionsAttempt.data, index);
  };

  const executeCommand = (command: AutocompleteCommand) => {
    switch (command.kind) {
      case 'command.unknown': {
        const params = routing.parseClusterUri(
          workspacesService.getActiveWorkspace()?.localClusterUri
        ).params;
        // ugly hack but QuickInput will be removed in v13
        if (inputValue.startsWith('tsh proxy db')) {
          usageService.captureProtocolUse(
            workspacesService.getRootClusterUri(),
            'db',
            'search_bar'
          );
        }
        documentsService.openNewTerminal({
          initCommand: inputValue,
          rootClusterId: routing.parseClusterUri(
            workspacesService.getRootClusterUri()
          ).params.rootClusterId,
          leafClusterId: params.leafClusterId,
        });
        break;
      }
      case 'command.tsh-ssh': {
        const { localClusterUri } = workspacesService.getActiveWorkspace();

        commandLauncher.executeCommand('tsh-ssh', {
          loginHost: command.loginHost,
          localClusterUri,
          origin: 'search_bar',
        });
        break;
      }
      case 'command.tsh-install': {
        commandLauncher.executeCommand('tsh-install', undefined);
        break;
      }
      case 'command.tsh-uninstall': {
        commandLauncher.executeCommand('tsh-uninstall', undefined);
        break;
      }
      default: {
        assertUnreachable(command);
      }
    }

    quickInputService.clearInputValueAndHide();
  };

  const pickSuggestion = (
    targetToken: AutocompleteToken,
    suggestions: Suggestion[],
    index?: number
  ) => {
    const suggestion = suggestions[index];

    setActiveSuggestion(index);
    quickInputService.pickSuggestion(targetToken, suggestion);
  };

  const onEscape = () => {
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
    [openCommandBarShortcutAction]: () => {
      quickInputService.show();
    },
  });

  // Reset active suggestion when the suggestion list changes.
  // We extract just the tokens and stringify the list to avoid stringifying big objects.
  // See https://github.com/facebook/react/issues/14476#issuecomment-471199055
  // TODO(ravicious): Remove the unnecessary effect.
  // https://beta.reactjs.org/learn/you-might-not-need-an-effect#chains-of-computations
  // https://beta.reactjs.org/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes
  useEffect(() => {
    setActiveSuggestion(0);
  }, [
    // We want to reset the active suggestion only if the
    // suggestions have changed.
    suggestionsAttempt.data?.map(suggestion => suggestion.token).join(','),
  ]);

  return {
    visible,
    suggestionsAttempt,
    activeSuggestion,
    inputValue,
    onFocus,
    onEscape,
    onEnter,
    onActiveSuggestion,
    onInputChange: quickInputService.setInputValue,
    onHide: quickInputService.hide,
    onShow: quickInputService.show,
    keyboardShortcut: getAccelerator(openCommandBarShortcutAction),
  };
}

export type State = ReturnType<typeof useQuickInput>;
