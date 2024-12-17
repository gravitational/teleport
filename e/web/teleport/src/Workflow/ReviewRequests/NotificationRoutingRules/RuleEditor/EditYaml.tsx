import { Flex, Box } from 'design';
import { State as Attempt } from 'shared/hooks/useAttemptNext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import TextEditor from 'shared/components/TextEditor';

import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';
import { AccessMonitoringRuleWithYaml } from 'e-teleport/services/accessmonitoringrule/types';

import {
  EditorSaveCancelButton,
  getDefaultPluginNotificationMessage,
} from './Shared';
import { YamlEditor } from './yamleditor';

export const EditYaml = ({
  selectedRule,
  ruleName,
  onEdit,
  onCancel,
  yamlEditor,
  onYamlEditorChange,
  fetchAttempt,
}: {
  selectedRule: AccessMonitoringRuleWithYaml;
  ruleName: string;
  onEdit(r: AccessMonitoringRuleWithYaml): void;
  onCancel(): void;
  yamlEditor: YamlEditor;
  onYamlEditorChange(y: YamlEditor): void;
  fetchAttempt: Attempt;
}) => {
  const isEditing = !!selectedRule;
  const { clusterId } = useStickyClusterId();
  const { attempt, run } = fetchAttempt;

  function handleSetYaml(newContent) {
    onYamlEditorChange({
      isDirty: selectedRule?.yaml !== newContent,
      content: newContent,
    });
  }

  function onSave() {
    if (isEditing) {
      run(() =>
        accessMonitoringRuleService
          .updateAccessMonitoringRule(
            { clusterId, name: ruleName },
            {
              yaml: yamlEditor.content,
            }
          )
          .then(onEdit)
      );
    } else {
      run(() =>
        accessMonitoringRuleService
          .createAccessMonitoringRule(clusterId, {
            yaml: yamlEditor.content,
          })
          .then(onEdit)
      );
    }
  }

  return (
    <Box data-testid="yaml">
      <Flex height="400px" my={5}>
        <TextEditor
          readOnly={false}
          data={[{ content: yamlEditor.content, type: 'yaml' }]}
          onChange={handleSetYaml}
        />
      </Flex>
      {getDefaultPluginNotificationMessage()}
      <EditorSaveCancelButton
        onSave={onSave}
        onCancel={onCancel}
        disabled={attempt.status === 'processing' || !yamlEditor.isDirty}
        isEditing={isEditing}
      />
    </Box>
  );
};
