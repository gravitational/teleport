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
