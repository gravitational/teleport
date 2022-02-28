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

import { useMemo } from 'react';
import AppContext from 'teleterm/ui/appContext';
import {
  KeyboardShortcutHandlers,
  useKeyboardShortcuts,
} from 'teleterm/ui/services/keyboardShortcuts';
import { useAppContext } from 'teleterm/ui/appContextProvider';

export function useTabShortcuts() {
  const ctx = useAppContext();
  const tabsShortcuts = useMemo(() => buildTabsShortcuts(ctx), [ctx]);
  useKeyboardShortcuts(tabsShortcuts);
}

function buildTabsShortcuts(ctx: AppContext): KeyboardShortcutHandlers {
  const handleTabIndex = (index: number) => () => {
    const docs = ctx.docsService.getDocuments();
    if (docs[index]) {
      ctx.docsService.open(docs[index].uri);
    }
  };

  const handleActiveTabClose = () => {
    const { uri } = ctx.docsService.getActive();
    ctx.docsService.close(uri);
  };

  const handleNewTabOpen = () => {
    ctx.docsService.openNewTerminal();
  };

  const handleTabSwitch = (direction: 'previous' | 'next') => () => {
    const activeDoc = ctx.docsService.getActive();
    const allDocuments = ctx.docsService
      .getDocuments()
      .filter(d => d.kind !== 'doc.home');
    const activeDocIndex = allDocuments.indexOf(activeDoc);
    const getPreviousIndex = () =>
      (activeDocIndex - 1 + allDocuments.length) % allDocuments.length;
    const getNextIndex = () => (activeDocIndex + 1) % allDocuments.length;
    const indexToOpen =
      direction === 'previous' ? getPreviousIndex() : getNextIndex();

    ctx.docsService.open(allDocuments[indexToOpen].uri);
  };
  return {
    'tab-1': handleTabIndex(1),
    'tab-2': handleTabIndex(2),
    'tab-3': handleTabIndex(3),
    'tab-4': handleTabIndex(4),
    'tab-5': handleTabIndex(5),
    'tab-6': handleTabIndex(6),
    'tab-7': handleTabIndex(7),
    'tab-8': handleTabIndex(8),
    'tab-9': handleTabIndex(9),
    'tab-close': handleActiveTabClose,
    'tab-previous': handleTabSwitch('previous'),
    'tab-next': handleTabSwitch('next'),
    'tab-new': handleNewTabOpen,
  };
}
