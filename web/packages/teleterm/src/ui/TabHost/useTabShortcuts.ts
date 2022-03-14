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
import {
  KeyboardShortcutHandlers,
  useKeyboardShortcuts,
} from 'teleterm/ui/services/keyboardShortcuts';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { useNewTabOpener } from 'teleterm/ui/TabHost/useNewTabOpener';

export function useTabShortcuts() {
  const { workspacesService } = useAppContext();
  workspacesService.useState();
  const documentService = workspacesService.getActiveWorkspaceDocumentService();
  const { openClusterTab } = useNewTabOpener();
  const tabsShortcuts = useMemo(
    () => buildTabsShortcuts(documentService, openClusterTab),
    [documentService]
  );
  useKeyboardShortcuts(tabsShortcuts);
}

function buildTabsShortcuts(
  documentService: DocumentsService,
  openClusterTab: () => void
): KeyboardShortcutHandlers {
  const handleTabIndex = (index: number) => () => {
    const docs = documentService.getDocuments();
    if (docs[index]) {
      documentService.open(docs[index].uri);
    }
  };

  const handleActiveTabClose = () => {
    const { uri } = documentService.getActive();
    documentService.close(uri);
  };

  const handleTabSwitch = (direction: 'previous' | 'next') => () => {
    const activeDoc = documentService.getActive();
    const allDocuments = documentService.getDocuments();
    const activeDocIndex = allDocuments.indexOf(activeDoc);
    const getPreviousIndex = () =>
      (activeDocIndex - 1 + allDocuments.length) % allDocuments.length;
    const getNextIndex = () => (activeDocIndex + 1) % allDocuments.length;
    const indexToOpen =
      direction === 'previous' ? getPreviousIndex() : getNextIndex();

    documentService.open(allDocuments[indexToOpen].uri);
  };
  return {
    'tab-1': handleTabIndex(0),
    'tab-2': handleTabIndex(1),
    'tab-3': handleTabIndex(2),
    'tab-4': handleTabIndex(3),
    'tab-5': handleTabIndex(4),
    'tab-6': handleTabIndex(5),
    'tab-7': handleTabIndex(6),
    'tab-8': handleTabIndex(7),
    'tab-9': handleTabIndex(8),
    'tab-close': handleActiveTabClose,
    'tab-previous': handleTabSwitch('previous'),
    'tab-next': handleTabSwitch('next'),
    'tab-new': openClusterTab,
  };
}
