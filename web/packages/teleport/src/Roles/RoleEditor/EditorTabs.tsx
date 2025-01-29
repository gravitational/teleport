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

import * as Icon from 'design/Icon';
import { SlideTabs } from 'design/SlideTabs';

export enum EditorTab {
  Standard,
  Yaml,
}

export const EditorTabs = ({
  onTabChange,
  selectedEditorTab,
  disabled,
  standardEditorId,
  yamlEditorId,
}: {
  onTabChange(t: EditorTab): void;
  selectedEditorTab: EditorTab;
  disabled: boolean;
  standardEditorId: string;
  yamlEditorId: string;
}) => {
  const standardLabel = 'Switch to standard editor';
  const yamlLabel = 'Switch to YAML editor';
  return (
    <SlideTabs
      appearance="round"
      tabs={[
        {
          key: 'standard',
          icon: Icon.ListAddCheck,
          tooltip: { content: standardLabel, position: 'bottom' },
          ariaLabel: standardLabel,
          controls: standardEditorId,
        },
        {
          key: 'yaml',
          icon: Icon.Code,
          tooltip: { content: yamlLabel, position: 'bottom' },
          ariaLabel: yamlLabel,
          controls: yamlEditorId,
        },
      ]}
      onChange={onTabChange}
      size="small"
      fitContent
      activeIndex={selectedEditorTab}
      disabled={disabled}
    />
  );
};
