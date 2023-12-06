/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useMemo } from 'react';

import {
  KeyboardShortcutHandlers,
  useKeyboardShortcuts,
} from 'teleterm/ui/services/keyboardShortcuts';
import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { useNewTabOpener } from 'teleterm/ui/TabHost/useNewTabOpener';
import { ClusterUri } from 'teleterm/ui/uri';

export function useTabShortcuts({
  documentsService,
  localClusterUri,
}: {
  documentsService: DocumentsService;
  localClusterUri: ClusterUri;
}) {
  const { openClusterTab, openTerminalTab } = useNewTabOpener({
    documentsService,
    localClusterUri,
  });
  const tabsShortcuts = useMemo(
    () => buildTabsShortcuts(documentsService, openClusterTab, openTerminalTab),
    [documentsService, openClusterTab, openTerminalTab]
  );
  useKeyboardShortcuts(tabsShortcuts);
}

function buildTabsShortcuts(
  documentService: DocumentsService,
  openClusterTab: () => void,
  openTerminalTab: () => void
): KeyboardShortcutHandlers {
  const handleTabIndex = (index: number) => () => {
    const docs = documentService.getDocuments();
    if (docs[index]) {
      documentService.open(docs[index].uri);
    }
  };

  const handleActiveTabClose = () => {
    const activeDocument = documentService.getActive();
    if (activeDocument) {
      documentService.close(activeDocument.uri);
    }
  };

  const handleTabSwitch = (direction: 'previous' | 'next') => () => {
    const activeDoc = documentService.getActive();
    const allDocuments = documentService.getDocuments();

    if (allDocuments.length === 0) {
      return;
    }

    const activeDocIndex = allDocuments.indexOf(activeDoc);
    const getPreviousIndex = () =>
      (activeDocIndex - 1 + allDocuments.length) % allDocuments.length;
    const getNextIndex = () => (activeDocIndex + 1) % allDocuments.length;
    const indexToOpen =
      direction === 'previous' ? getPreviousIndex() : getNextIndex();

    documentService.open(allDocuments[indexToOpen].uri);
  };

  return {
    tab1: handleTabIndex(0),
    tab2: handleTabIndex(1),
    tab3: handleTabIndex(2),
    tab4: handleTabIndex(3),
    tab5: handleTabIndex(4),
    tab6: handleTabIndex(5),
    tab7: handleTabIndex(6),
    tab8: handleTabIndex(7),
    tab9: handleTabIndex(8),
    closeTab: handleActiveTabClose,
    previousTab: handleTabSwitch('previous'),
    nextTab: handleTabSwitch('next'),
    newTab: openClusterTab,
    newTerminalTab: openTerminalTab,
  };
}
