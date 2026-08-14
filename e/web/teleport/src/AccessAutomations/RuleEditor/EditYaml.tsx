import { Flex } from 'design';
import TextEditor from 'shared/components/TextEditor';
import { State as Attempt } from 'shared/hooks/useAttemptNext';

import { AccessMonitoringRuleWithYaml } from 'e-teleport/services/accessmonitoringrule/types';

import {
  EditorSaveCancelButton,
  getDefaultPluginNotificationMessage,
} from './Shared';
import { YamlEditor } from './yamleditor';

export const EditYaml = ({
  selectedRule,
  onEdit,
  onCancel,
  yamlEditor,
  onYamlEditorChange,
  fetchAttempt,
  onSave,
}: {
  selectedRule: AccessMonitoringRuleWithYaml;
  onEdit(r: Partial<AccessMonitoringRuleWithYaml>): void;
  onCancel(): void;
  yamlEditor: YamlEditor;
  onYamlEditorChange(y: YamlEditor): void;
  fetchAttempt: Attempt;
  onSave(r: Partial<AccessMonitoringRuleWithYaml>): void;
}) => {
  const isEditing = !!selectedRule;
  const { attempt, run } = fetchAttempt;

  function handleSetYaml(newContent) {
    onYamlEditorChange({
      isDirty: selectedRule?.yaml !== newContent,
      content: newContent,
    });
  }

  function handleSave() {
    if (isEditing) {
      run(async () => onEdit({ yaml: yamlEditor.content }));
    } else {
      run(async () => onSave({ yaml: yamlEditor.content }));
    }
  }

  return (
    <Flex data-testid="yaml" flex="1" flexDirection="column">
      <Flex height="400px" my={5} flex="1" flexDirection="column">
        <TextEditor
          readOnly={false}
          data={[{ content: yamlEditor.content, type: 'yaml' }]}
          onChange={handleSetYaml}
        />
      </Flex>
      {getDefaultPluginNotificationMessage()}
      <EditorSaveCancelButton
        onSave={handleSave}
        onCancel={onCancel}
        disabled={attempt.status === 'processing' || !yamlEditor.isDirty}
        isEditing={isEditing}
      />
    </Flex>
  );
};
