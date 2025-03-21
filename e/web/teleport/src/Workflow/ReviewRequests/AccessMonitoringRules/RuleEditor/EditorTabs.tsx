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
  disabled,
}: {
  onTabChange(t: EditorTab): void;
  selectedEditorTab: EditorTab;
  isProcessing: boolean;
  disabled: boolean;
}) => {
  return (
    <SlideTabs
      appearance="round"
      tabs={tabs}
      onChange={onTabChange}
      size="medium"
      activeIndex={selectedEditorTab}
      isProcessing={isProcessing}
      disabled={disabled}
    />
  );
};
