import React, { useState } from 'react';
import { Plugin } from 'teleport/services/integrations';
import useAttempt from 'shared/hooks/useAttemptNext';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import { Alert } from 'design';
import { yamlService } from 'teleport/services/yaml/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import { getErrMessage } from 'shared/utils/errorType';
import useTeleport from 'teleport/useTeleport';

import {
  AccessMonitoringRule,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';

import { EditStandard } from './EditStandard';
import { EditYaml } from './EditYaml';
import { RequiresEnrollingPlugin } from './RequiresEnrollingPlugin';
import { getRuleCondition } from './rulecondition';

import { Sidebar, EditorWrapper } from './Shared';
import { EditorTab, EditorTabs } from './EditorTabs';
import { EditorHeader } from './EditorHeader';
import {
  buildRuleFromStandardEditor,
  getConfigurableFieldsForStandardEditor,
  newAccessMonitoringRule,
  StandardEditor,
} from './standardeditor';
import { newYamlRuleFromTemplate, YamlEditor } from './yamleditor';
import { RequiresResetToStandard } from './RequiresResetToStandard';

export const RuleEditor = ({
  selectedRule,
  onCancel,
  onEdit,
  plugins,
  onDelete,
}: {
  // selectedRule can be null if a user is creaitng
  // a new rule instead.
  selectedRule?: AccessMonitoringRuleWithYaml;
  onCancel(): void;
  onEdit(r: AccessMonitoringRuleWithYaml): void;
  plugins: Plugin[];
  onDelete(r: AccessMonitoringRule): void;
}) => {
  const ctx = useTeleport();
  const fetchAttempt = useAttempt('');
  const { attempt, setAttempt } = fetchAttempt;

  const [standardEditor, setStandardEditor] = useState<StandardEditor>({
    rule: selectedRule ? selectedRule.object : newAccessMonitoringRule(),
    ...getConfigurableFieldsForStandardEditor(selectedRule?.object, plugins),
    isDirty: false,
  });

  const [yamlEditor, setYamlEditor] = useState<YamlEditor>({
    content: selectedRule?.yaml ?? '',
    isDirty: false,
  });

  // Defaults to yaml editor if the rule condition could not be parsed.
  const [selectedEditorTab, setSelectedEditorTab] = useState<EditorTab>(() =>
    !standardEditor.ruleCondition ? EditorTab.Yaml : EditorTab.Standard
  );

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
    setStandardEditor({
      ...standardEditor,
      ruleCondition: getRuleCondition(''),
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

    const configurableFields = getConfigurableFieldsForStandardEditor(
      parsedRule,
      plugins
    );

    setStandardEditor({
      rule: parsedRule,
      isDirty: yamlEditor.isDirty,
      ...configurableFields,
    });

    // If the rule condition returns null, it means the
    // condition couldn't be parsed.
    if (!configurableFields.ruleCondition) {
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
        if (!yamlEditor.content) {
          const template = newYamlRuleFromTemplate(standardEditor);
          setYamlEditor({
            content: template,
            isDirty: true,
            requiresReset: !standardEditor.ruleCondition,
          });
        } else {
          const yamlified = await yamlilfyRule();
          if (!yamlified) {
            return;
          }
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
    hasPluginAccess && isCreating && plugins.length === 0;
  return (
    <Sidebar p={4}>
      <EditorHeader
        rule={selectedRule?.object}
        onDelete={onDelete}
        requiresEnrollingPlugins={requiresEnrollingPlugins}
        onCancel={onCancel}
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
            {!standardEditor.ruleCondition && (
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
            />
          </>
        )}
        {selectedEditorTab === EditorTab.Yaml && (
          <EditYaml
            yamlEditor={yamlEditor}
            onYamlEditorChange={setYamlEditor}
            onEdit={onEdit}
            ruleName={standardEditor.ruleName}
            onCancel={onCancel}
            selectedRule={selectedRule}
            fetchAttempt={fetchAttempt}
          />
        )}
      </EditorWrapper>
    </Sidebar>
  );
};
