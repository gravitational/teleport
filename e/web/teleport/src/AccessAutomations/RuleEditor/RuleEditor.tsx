import { useState } from 'react';

import { Alert } from 'design';
import Dialog from 'design/Dialog';
import useAttempt from 'shared/hooks/useAttemptNext';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { getErrMessage } from 'shared/utils/errorType';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleType,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';
import { Plugin } from 'teleport/services/integrations';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import { yamlService } from 'teleport/services/yaml/yaml';
import useTeleport from 'teleport/useTeleport';

import { EditorHeader } from './EditorHeader';
import { EditorTab, EditorTabs } from './EditorTabs';
import { EditStandard } from './EditStandard';
import { EditYaml } from './EditYaml';
import { RequiresEnrollingPlugin } from './RequiresEnrollingPlugin';
import { RequiresResetToStandard } from './RequiresResetToStandard';
import { EditorWrapper } from './Shared';
import {
  buildRuleFromStandardEditor,
  ConfigurableFieldsForStandardEditor,
  getConfigurableNotificationFieldsForStandardEditor,
  getConfigurableReviewFieldsForStandardEditor,
  newAccessMonitoringRule,
  StandardEditor,
} from './standardeditor';
import { YamlEditor } from './yamleditor';

export const RuleEditor = ({
  selectedRule,
  onCancel,
  onEdit,
  plugins,
  onDelete,
  editor,
  onSave,
}: {
  // selectedRule can be null if a user is creaitng
  // a new rule instead.
  selectedRule?: AccessMonitoringRuleWithYaml;
  onCancel(): void;
  onEdit(r: Partial<AccessMonitoringRuleWithYaml>): void;
  plugins: Plugin[];
  onDelete(): void;
  editor: AccessMonitoringRuleType;
  onSave(r: Partial<AccessMonitoringRuleWithYaml>): void;
}) => {
  const ctx = useTeleport();
  const fetchAttempt = useAttempt('');
  const { attempt, setAttempt } = fetchAttempt;

  const [standardEditor, setStandardEditor] = useState<StandardEditor>(() => {
    let standardEditor = {
      rule: selectedRule ? selectedRule.object : newAccessMonitoringRule(),
      isDirty: false,
    };

    if (editor === AccessMonitoringRuleType.Review) {
      return {
        ...standardEditor,
        ...getConfigurableReviewFieldsForStandardEditor(selectedRule?.object),
      };
    }

    // Return notification fields by default.
    return {
      ...standardEditor,
      ...getConfigurableNotificationFieldsForStandardEditor(
        selectedRule?.object,
        plugins
      ),
    };
  });

  const [yamlEditor, setYamlEditor] = useState<YamlEditor>({
    content: selectedRule?.yaml ?? '',
    isDirty: false,
  });

  // Defaults to yaml editor if the rule condition could not be parsed.
  const [selectedEditorTab, setSelectedEditorTab] = useState<EditorTab>(() =>
    standardEditor.errors?.length > 0 ? EditorTab.Yaml : EditorTab.Standard
  );

  function resetForNotificationsEditor() {
    let configurableFields = getConfigurableNotificationFieldsForStandardEditor(
      null,
      plugins
    );
    configurableFields = {
      ...configurableFields,
      ruleName: standardEditor.ruleName || configurableFields.ruleName,
      pluginOption: standardEditor.pluginOption?.value
        ? standardEditor.pluginOption
        : configurableFields.pluginOption,
    };
    return configurableFields;
  }

  function resetForReviewsEditor(): ConfigurableFieldsForStandardEditor {
    let configurableFields = getConfigurableReviewFieldsForStandardEditor(null);
    configurableFields = {
      ...configurableFields,
      ruleName: standardEditor.ruleName || configurableFields.ruleName,
      desiredState:
        standardEditor.desiredState || configurableFields.desiredState,
      pluginOption: standardEditor.pluginOption?.value
        ? standardEditor.pluginOption
        : configurableFields.pluginOption,
      automaticReview: standardEditor.automaticReview?.value
        ? standardEditor.automaticReview
        : configurableFields.automaticReview,
      reviewDecisionOption: standardEditor.reviewDecisionOption?.value
        ? standardEditor.reviewDecisionOption
        : configurableFields.reviewDecisionOption,
      schedule: standardEditor.schedule || configurableFields.schedule,
    };
    return configurableFields;
  }

  /**
   * resets the standard editor back into viewable state by setting
   * the ruleCondition field back into a parsable default value.
   *
   * In the standard editor, ruleCondition is a field that we "try"
   * to display in a friendly UX manner, IF it can be parsed
   * in the way the UI expects it to. If it cannot be parsed, then
   * we disable the standard editor and let user know they need to
   * "reset" to view the standard editor.
   */
  function resetForStandardEditor() {
    setYamlEditor({ ...yamlEditor, requiresReset: false });

    const configurableFields =
      editor === AccessMonitoringRuleType.Notification
        ? resetForNotificationsEditor()
        : resetForReviewsEditor();

    setStandardEditor({
      rule: newAccessMonitoringRule(),
      ...configurableFields,
      errors: [],
      isDirty: false,
    });
  }

  async function parseYaml() {
    setAttempt({ status: 'processing' });
    let parsedRule: AccessMonitoringRule;
    try {
      // Convert yaml back into js object.
      parsedRule = await yamlService.parse<AccessMonitoringRule>(
        YamlSupportedResourceKind.AccessMonitoringRule,
        { yaml: yamlEditor.content }
      );
      setAttempt({ status: 'success' });
    } catch (err) {
      setAttempt({ status: 'failed', statusText: getErrMessage(err) });
      return false;
    }

    const configurableFields =
      editor === AccessMonitoringRuleType.Notification
        ? getConfigurableNotificationFieldsForStandardEditor(
            parsedRule,
            plugins
          )
        : getConfigurableReviewFieldsForStandardEditor(parsedRule);

    setStandardEditor({
      rule: parsedRule,
      isDirty: yamlEditor.isDirty,
      ...configurableFields,
    });

    if (configurableFields.errors?.length > 0) {
      setYamlEditor({ ...yamlEditor, requiresReset: true });
    }

    return true;
  }

  async function yamlilfyRule() {
    setAttempt({ status: 'processing' });
    let yamilfiedRule: string = '';
    try {
      // Convert js object back into yaml string
      yamilfiedRule = await yamlService.stringify<AccessMonitoringRule>(
        YamlSupportedResourceKind.AccessMonitoringRule,
        { resource: buildRuleFromStandardEditor(standardEditor) }
      );
      setAttempt({ status: 'success' });
    } catch (err) {
      setAttempt({
        status: 'failed',
        statusText: getErrMessage(err),
      });
      return false;
    }

    setYamlEditor({
      content: yamilfiedRule,
      isDirty: selectedRule?.yaml != yamilfiedRule,
    });

    return true;
  }

  async function onTabChange(activeIndex: EditorTab) {
    switch (activeIndex) {
      case EditorTab.Standard: {
        if (!yamlEditor.content) {
          //  nothing to parse.
          return;
        }
        const parsed = await parseYaml();
        if (!parsed) {
          return;
        }
        break;
      }
      case EditorTab.Yaml: {
        if (yamlEditor.requiresReset) {
          break;
        }
        const yamlified = await yamlilfyRule();
        if (!yamlified) {
          return;
        }
        break;
      }
      default:
        assertUnreachable(activeIndex);
    }

    setSelectedEditorTab(activeIndex);
  }

  const isCreating = !selectedRule?.object;
  const hasPluginAccess = ctx.storeUser.getPluginsAccess().read;
  const requiresEnrollingPlugins =
    hasPluginAccess &&
    isCreating &&
    plugins.length === 0 &&
    editor === AccessMonitoringRuleType.Notification; // plugin is only required for notification rules.

  return (
    <Dialog
      dialogCss={() => ({
        height: '80%',
        width: '80%',
        maxHeight: '1000px',
        maxWidth: '1400px',
      })}
      onClose={onCancel}
      open={true}
    >
      <EditorHeader
        rule={selectedRule?.object}
        onDelete={onDelete}
        requiresEnrollingPlugins={requiresEnrollingPlugins}
        onCancel={onCancel}
        editor={editor}
      />
      {requiresEnrollingPlugins && <RequiresEnrollingPlugin />}
      {attempt.status === 'failed' && (
        <Alert children={attempt.statusText} mt={3} />
      )}
      <EditorWrapper mute={requiresEnrollingPlugins}>
        <EditorTabs
          onTabChange={onTabChange}
          selectedEditorTab={selectedEditorTab}
          isProcessing={attempt.status === 'processing'}
          disabled={requiresEnrollingPlugins}
        />
        {selectedEditorTab === EditorTab.Standard && (
          <>
            {standardEditor.errors?.length > 0 && (
              <RequiresResetToStandard reset={resetForStandardEditor} />
            )}
            <EditStandard
              selectedRule={selectedRule}
              onEdit={onEdit}
              plugins={plugins}
              onCancel={onCancel}
              standardEditor={standardEditor}
              onStandardEditorChange={setStandardEditor}
              fetchAttempt={fetchAttempt}
              yamlIsDirty={yamlEditor.isDirty}
              editor={editor}
              onSave={onSave}
            />
          </>
        )}
        {selectedEditorTab === EditorTab.Yaml && (
          <EditYaml
            yamlEditor={yamlEditor}
            onYamlEditorChange={setYamlEditor}
            onEdit={onEdit}
            onCancel={onCancel}
            selectedRule={selectedRule}
            fetchAttempt={fetchAttempt}
            onSave={onSave}
          />
        )}
      </EditorWrapper>
    </Dialog>
  );
};
