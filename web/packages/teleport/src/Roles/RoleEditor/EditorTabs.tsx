/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React from 'react';

import { SlideTabs } from 'design/SlideTabs';

const tabs = ['Standard', 'YAML'];
export enum EditorTab {
  Standard,
  Yaml,
}

export const EditorTabs = ({
  onTabChange,
  selectedEditorTab,
  isProcessing,
}: {
  onTabChange(t: EditorTab): void;
  selectedEditorTab: EditorTab;
  isProcessing: boolean;
}) => {
  return (
    <SlideTabs
      appearance="round"
      tabs={tabs}
      onChange={onTabChange}
      size="medium"
      activeIndex={selectedEditorTab}
      isProcessing={isProcessing}
    />
  );
};
