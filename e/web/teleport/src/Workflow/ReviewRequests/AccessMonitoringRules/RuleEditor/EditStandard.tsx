import { components } from 'react-select';

import { Box, Mark, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { FieldSelectCreatableAsync } from 'shared/components/FieldSelect/FieldSelectCreatable';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { State as Attempt } from 'shared/hooks/useAttemptNext';

import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';
import { AccessMonitoringRuleWithYaml } from 'e-teleport/services/accessmonitoringrule/types';
import { Plugin } from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import {
  AccessRequestMatchCondition,
  AccessRequestMatchConditionOption,
  accessRequestMatchConditionOptions,
  RuleCondition,
} from './rulecondition';
import {
  EditorSaveCancelButton,
  EditorWrapper,
  getDefaultPluginNotificationMessage,
} from './Shared';
import {
  buildRuleFromStandardEditor,
  ConfigurableFieldsForStandardEditor,
  hasModifiedFields,
  StandardEditor,
} from './standardeditor';

export const EditStandard = ({
  selectedRule,
  onEdit,
  onCancel,
  plugins,
  standardEditor,
  onStandardEditorChange,
  fetchAttempt,
  yamlIsDirty,
}: {
  selectedRule: AccessMonitoringRuleWithYaml;
  onEdit(r: AccessMonitoringRuleWithYaml): void;
  onCancel(): void;
  plugins: Plugin[];
  standardEditor: StandardEditor;
  onStandardEditorChange(s: StandardEditor): void;
  fetchAttempt: Attempt;
  /**
   * There's an edge case where the user can add more
   * fields to the rule resource from the yaml editor
   * that the standard editor does not read, so if the
   * yaml is dirty we assume it is always dirty to account
   * for this edge case.
   */
  yamlIsDirty: boolean;
}) => {
  const isEditing = !!selectedRule;
  const ctx = useTeleport();
  const { attempt, run } = fetchAttempt;
  const { clusterId } = useStickyClusterId();
  const { rule, ...configurableFields } = standardEditor;
  const { ruleCondition, ruleName, recipients, pluginOption } =
    configurableFields;

  function onSave(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    if (isEditing) {
      run(() =>
        accessMonitoringRuleService
          .updateAccessMonitoringRule(
            { name: rule.metadata.name, clusterId },
            {
              object: buildRuleFromStandardEditor(standardEditor),
            }
          )
          .then(onEdit)
      );
    } else {
      run(() =>
        accessMonitoringRuleService
          .createAccessMonitoringRule(clusterId, {
            object: buildRuleFromStandardEditor(standardEditor),
          })
          .then(onEdit)
      );
    }
  }

  async function fetchRoleOptions(search: string): Promise<Option[]> {
    const roleAccess = ctx.storeUser.getRoleAccess();
    if (roleAccess.list && roleAccess.read) {
      const roles = await ctx.resourceService.fetchRoles({ search, limit: 50 });
      return roles.items.map(r => ({ value: r.name, label: r.name }));
    }

    return [];
  }

  function getLastFilter() {
    if (ruleCondition?.values?.length > 1) {
      const lastIndex = ruleCondition.values.length - 1;
      return ruleCondition.values[lastIndex].value;
    }
    return '';
  }

  function handleStandardEditorChange(
    modified: Partial<ConfigurableFieldsForStandardEditor>
  ) {
    const updatedFields: ConfigurableFieldsForStandardEditor = {
      ruleName:
        modified.ruleName === null
          ? ''
          : modified.ruleName || standardEditor.ruleName,
      pluginOption: modified.pluginOption || standardEditor.pluginOption,
      ruleCondition: modified.ruleCondition || standardEditor.ruleCondition,
      recipients: modified.recipients || standardEditor.recipients,
    };

    onStandardEditorChange({
      ...standardEditor,
      ...updatedFields,
      isDirty: hasModifiedFields(
        updatedFields,
        selectedRule?.object,
        yamlIsDirty
      ),
    });
  }

  let notifyStateComponent: JSX.Element;
  switch (ruleCondition?.field?.value) {
    case AccessRequestMatchCondition.Roles: {
      notifyStateComponent = (
        <SelectCreateRoles
          getLastFilter={getLastFilter}
          fetchRoleOptions={fetchRoleOptions}
          onChangeRuleConditionValues={(values: Option[]) => {
            handleStandardEditorChange({
              ruleCondition: {
                ...ruleCondition,
                values: values || [],
              },
            });
          }}
          ruleCondition={ruleCondition}
          isDisabled={attempt.status === 'processing'}
        />
      );
      break;
    }
    // AnyXXX cases does not require a user to select or create
    // anything.
    case AccessRequestMatchCondition.AnyRoles:
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          <EditorWrapper mute={!ruleCondition} data-testid="standard">
            <Box mt={2}>
              <FieldInput
                label="Rule Name"
                placeholder="name"
                value={ruleName}
                onChange={e =>
                  handleStandardEditorChange({
                    ruleName: e.target.value || null,
                  })
                }
                rule={requiredField('Rule name is required')}
                readonly={isEditing || attempt.status === 'processing'}
              />
              <Box mb={4}>
                <Text bold mb={2}>
                  Match Condition
                </Text>
                <Box>
                  <FieldSelect
                    width="100%"
                    mb={1}
                    label="Notification Type"
                    options={accessRequestMatchConditionOptions}
                    isDisabled={attempt.status === 'processing'}
                    onChange={(o: AccessRequestMatchConditionOption) =>
                      handleStandardEditorChange({
                        ruleCondition: { field: o, values: undefined },
                      })
                    }
                    value={ruleCondition?.field}
                  />
                  {notifyStateComponent}
                </Box>
              </Box>
              <Box>
                <Text bold mb={2}>
                  Recipients
                </Text>
                <FieldSelect
                  label="The integration to apply this rule to"
                  isSearchable={true}
                  options={plugins.map(p => ({ value: p.name, label: p.name }))}
                  onChange={(o: Option) =>
                    handleStandardEditorChange({ pluginOption: o })
                  }
                  value={pluginOption}
                  isDisabled={attempt.status === 'processing'}
                  placeholder="Select an integration"
                  mb={1}
                  rule={requiredField<Option>('An integration is required')}
                />
                <FieldSelectCreatable
                  inputId="recipients"
                  isDisabled={attempt.status === 'processing'}
                  isMulti
                  isClearable
                  isSearchable
                  label="Recipients to notify"
                  placeholder="Start typing and press enter"
                  onChange={(opts: Option[]) =>
                    handleStandardEditorChange({ recipients: opts || [] })
                  }
                  options={recipients ?? []}
                  value={recipients ?? []}
                  toolTipContent={
                    configurableFields.pluginOption
                      ? getRecipientIconTooltip(
                          configurableFields.pluginOption.value
                        )
                      : null
                  }
                />
              </Box>
              {standardEditor.pluginOption?.value &&
                getDefaultPluginNotificationMessage(
                  standardEditor.pluginOption.value
                )}
            </Box>
          </EditorWrapper>
          <EditorSaveCancelButton
            onSave={() => onSave(validator)}
            onCancel={onCancel}
            disabled={
              attempt.status === 'processing' ||
              !ruleCondition ||
              !standardEditor.isDirty
            }
            isEditing={isEditing}
          />
        </>
      )}
    </Validation>
  );
};

const SelectCreateRoles = ({
  getLastFilter,
  fetchRoleOptions,
  onChangeRuleConditionValues,
  ruleCondition,
  isDisabled,
}: {
  getLastFilter(): string;
  fetchRoleOptions(s: string): Promise<Option[]>;
  onChangeRuleConditionValues(values: Option[]): void;
  ruleCondition: RuleCondition;
  isDisabled: boolean;
}) => {
  return (
    <FieldSelectCreatableAsync
      inputId="roles"
      width="100%"
      placeholder="Start typing a role name and press enter"
      noOptionsMessage={() => 'Start typing a role name and press enter'}
      label="Name of roles to match"
      rule={requiredField('At least one role name is required')}
      isMulti
      isClearable
      isSearchable
      components={{
        MultiValueContainer,
      }}
      customProps={{
        lastFilter: getLastFilter(),
      }}
      options={[]}
      loadOptions={async input => await fetchRoleOptions(input)}
      isDisabled={isDisabled}
      onChange={onChangeRuleConditionValues}
      value={ruleCondition.values}
      defaultOptions={true}
    />
  );
};

function getRecipientIconTooltip(pluginName: string) {
  const lowerCasedName = pluginName.toLowerCase();
  if (lowerCasedName.includes('slack')) {
    return (
      <>
        Recipients can be emails and channel names. For any channels you define,
        you will need to <Mark>/invite</Mark> the integration to those channels.
      </>
    );
  }
  if (lowerCasedName.includes('mattermost')) {
    return (
      <>
        Recipients can be emails and team/channel names. You must define
        team/channel names with format{' '}
        <Mark>{'<team-name>/<channel-name>'}</Mark>. Make sure to invite the
        integration to every team/channel.
      </>
    );
  }
  if (lowerCasedName.includes('msteams')) {
    return (
      <>
        Recipients can be Microsoft Teams user emails/IDs, or channel URL
        recipients. Make sure to invite the bot to every team/channel.
      </>
    );
  }
  if (lowerCasedName.includes('datadog')) {
    return <>Recipients can be emails and team handle names.</>;
  }
  if (lowerCasedName.includes('email')) {
    return <>Recipients can be emails.</>;
  }
}

const MultiValueContainer = (
  props: CustomSelectComponentProps<{ lastFilter: string }>
) => {
  const lastFilter = props.selectProps.customProps.lastFilter;
  const currFilter = props.data.value;

  const isLastFilter = lastFilter === currFilter;
  return (
    <>
      <components.MultiValueContainer {...props} />
      {lastFilter && !isLastFilter && <Text typography="body4">OR</Text>}
    </>
  );
};
