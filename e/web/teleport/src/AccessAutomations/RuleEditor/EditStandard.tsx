import { useState, type JSX } from 'react';
import { components } from 'react-select';

import { Box, Flex, Mark, Text } from 'design';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import FieldInput from 'shared/components/FieldInput';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { FieldSelectCreatableAsync } from 'shared/components/FieldSelect/FieldSelectCreatable';
import { newSchedule, ScheduleEditor } from 'shared/components/ScheduleEditor';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';
import { TraitsEditor, TraitsOption } from 'shared/components/TraitsEditor';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { State as Attempt } from 'shared/hooks/useAttemptNext';

import {
  AccessMonitoringRuleType,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';
import { LabelsInput } from 'teleport/components/LabelsInput';
import { nonEmptyLabels } from 'teleport/components/LabelsInput/LabelsInput';
import { Plugin } from 'teleport/services/integrations';
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
  ReviewDecisionOption,
  reviewDecisionOptions,
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
  editor,
  onSave,
}: {
  selectedRule: AccessMonitoringRuleWithYaml;
  onEdit(r: Partial<AccessMonitoringRuleWithYaml>): void;
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
  editor: AccessMonitoringRuleType;
  onSave(r: Partial<AccessMonitoringRuleWithYaml>): void;
}) => {
  const isEditing = !!selectedRule;
  const ctx = useTeleport();
  const { attempt, run } = fetchAttempt;
  const { rule, ...configurableFields } = standardEditor;
  const {
    ruleCondition,
    ruleName,
    recipients,
    pluginOption,
    reviewDecisionOption,
    schedule,
    errors,
  } = configurableFields;

  const [notificationEnabled, setNotificationEnabled] = useState<boolean>(
    !!rule.spec?.notification?.name
  );

  const [scheduleEnabled, setScheduleEnabled] = useState<boolean>(
    !!schedule?.shifts && Object.keys(schedule.shifts).length > 0
  );

  function handleNotificationToggle(enabled: boolean) {
    // Clear notification configuration when disabled.
    if (!enabled) {
      partialStandardEditorChange({
        pluginOption: null,
        recipients: [],
      });
    }

    setNotificationEnabled(enabled);
  }

  function handleScheduleToggle(enabled: boolean) {
    // Clear schedule configuration when disabled.
    if (!enabled) {
      partialStandardEditorChange({
        schedule: null,
      });
    }

    setScheduleEnabled(enabled);
  }

  function handleSave(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    if (isEditing) {
      run(async () =>
        onEdit({ object: buildRuleFromStandardEditor(standardEditor) })
      );
    } else {
      run(async () =>
        onSave({ object: buildRuleFromStandardEditor(standardEditor) })
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
    if (ruleCondition?.rolesCondition?.values?.length > 1) {
      const lastIndex = ruleCondition.rolesCondition.values.length - 1;
      return ruleCondition.rolesCondition.values[lastIndex].value;
    }
    return '';
  }

  function partialStandardEditorChange(
    modified: Partial<ConfigurableFieldsForStandardEditor>
  ) {
    // We use strict null comparision for certain fields to allow explicit null
    // overrides, while missing or undefined fields fall back to existing values.
    const updatedFields: ConfigurableFieldsForStandardEditor = {
      ruleName:
        modified.ruleName === null
          ? ''
          : modified.ruleName || standardEditor.ruleName,
      pluginOption:
        modified.pluginOption === null
          ? null
          : modified.pluginOption || standardEditor.pluginOption,
      ruleCondition: modified.ruleCondition || standardEditor.ruleCondition,
      recipients: modified.recipients || standardEditor.recipients,
      automaticReview:
        modified.automaticReview || standardEditor.automaticReview,
      reviewDecisionOption:
        modified.reviewDecisionOption || standardEditor.reviewDecisionOption,
      desiredState: modified.desiredState || standardEditor.desiredState,
      schedule:
        modified.schedule === null
          ? null
          : modified.schedule || standardEditor.schedule,
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
  switch (ruleCondition?.rolesCondition?.field?.value) {
    case AccessRequestMatchCondition.MatchAnyRoles:
    case AccessRequestMatchCondition.MatchAllRoles: {
      notifyStateComponent = (
        <SelectCreateRoles
          getLastFilter={getLastFilter}
          fetchRoleOptions={fetchRoleOptions}
          onChangeRuleConditionValues={(values: Option[]) => {
            partialStandardEditorChange({
              ruleCondition: {
                ...ruleCondition,
                rolesCondition: {
                  ...ruleCondition.rolesCondition,
                  values: values || [],
                },
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
          <EditorWrapper mute={errors?.length > 0} data-testid="standard">
            <Box mt={2}>
              <FieldInput
                label="Rule Name"
                placeholder="name"
                value={ruleName}
                onChange={e =>
                  partialStandardEditorChange({
                    ruleName: e.target.value || null,
                  })
                }
                rule={requiredField('Rule name is required')}
                readonly={isEditing || attempt.status === 'processing'}
              />
              <Box mb={4}>
                <Text bold>Match Condition</Text>
                <Text mb={2}>
                  {editor === AccessMonitoringRuleType.Review && (
                    <>
                      Select one or more roles, and optionally select resource
                      labels or user traits, to define when this rule applies.
                    </>
                  )}
                </Text>
                <Box>
                  {editor === AccessMonitoringRuleType.Notification && (
                    <FieldSelect
                      width="100%"
                      mb={1}
                      label="Notification Type"
                      options={accessRequestMatchConditionOptions}
                      isDisabled={attempt.status === 'processing'}
                      onChange={(o: AccessRequestMatchConditionOption) =>
                        partialStandardEditorChange({
                          ruleCondition: {
                            ...ruleCondition,
                            rolesCondition: { field: o, values: undefined },
                          },
                        })
                      }
                      value={ruleCondition?.rolesCondition?.field}
                    />
                  )}
                  {notifyStateComponent}
                  <Box mb={2}>
                    <Text typography="body3" mb={2}>
                      Resource labels to match (Optional)
                    </Text>
                    <LabelsInput
                      adjective="resource label"
                      disableBtns={attempt.status === 'processing'}
                      labels={
                        standardEditor.ruleCondition?.resourcesCondition || []
                      }
                      setLabels={labels =>
                        partialStandardEditorChange({
                          ruleCondition: {
                            ...ruleCondition,
                            resourcesCondition: labels,
                          },
                        })
                      }
                      rule={nonEmptyLabels}
                    />
                  </Box>
                  {editor === AccessMonitoringRuleType.Review && (
                    <>
                      <Box mb={2}>
                        <Text typography="body3" mb={2}>
                          User traits to match (Optional)
                        </Text>
                        <TraitsEditor
                          label=""
                          isLoading={attempt.status === 'processing'}
                          configuredTraits={
                            standardEditor.ruleCondition?.traitsCondition || []
                          }
                          setConfiguredTraits={(o: TraitsOption[]) =>
                            partialStandardEditorChange({
                              ruleCondition: {
                                ...ruleCondition,
                                traitsCondition: o,
                              },
                            })
                          }
                        />
                      </Box>
                      <Box>
                        <FieldCheckbox
                          label="For specified times only"
                          size="small"
                          checked={scheduleEnabled}
                          onChange={e => handleScheduleToggle(e.target.checked)}
                        />
                        {scheduleEnabled && (
                          <Flex ml="28px">
                            <ScheduleEditor
                              schedule={schedule || newSchedule()}
                              setSchedule={schedule => {
                                partialStandardEditorChange({
                                  schedule: schedule,
                                });
                              }}
                            />
                          </Flex>
                        )}
                      </Box>
                    </>
                  )}
                </Box>
              </Box>
              {editor === AccessMonitoringRuleType.Review && (
                <Box mb={4}>
                  <Text bold>Automatic Review</Text>
                  <Text mb={2}>
                    Select an automatic review decision for matching access
                    requests.
                  </Text>
                  <FieldSelect
                    label="Review decision"
                    isSearchable={true}
                    options={reviewDecisionOptions}
                    onChange={(o: ReviewDecisionOption) =>
                      partialStandardEditorChange({ reviewDecisionOption: o })
                    }
                    value={reviewDecisionOption}
                    isDisabled={attempt.status === 'processing'}
                    placeholder="Select the desired access request state"
                    mb={1}
                    rule={requiredField<Option>(
                      'A review decision is required'
                    )}
                  />
                </Box>
              )}
              <Box>
                <Text bold mb={2}>
                  Notification Routing
                </Text>
                {editor === AccessMonitoringRuleType.Review && (
                  <FieldCheckbox
                    label="Enable notification routing"
                    size="small"
                    checked={notificationEnabled}
                    onChange={e => handleNotificationToggle(e.target.checked)}
                  />
                )}
                {(editor === AccessMonitoringRuleType.Notification ||
                  notificationEnabled) && (
                  <>
                    <FieldSelect
                      label="The integration to route notifications to"
                      isSearchable={true}
                      options={plugins.map(p => ({
                        value: p.name,
                        label: p.name,
                      }))}
                      onChange={(o: Option) =>
                        partialStandardEditorChange({ pluginOption: o })
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
                      label="Recipients to notify (Optional)"
                      placeholder="Start typing and press enter"
                      onChange={(opts: Option[]) =>
                        partialStandardEditorChange({ recipients: opts || [] })
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
                    {standardEditor.pluginOption?.value &&
                      getDefaultPluginNotificationMessage(
                        standardEditor.pluginOption.value
                      )}
                  </>
                )}
              </Box>
            </Box>
          </EditorWrapper>
          <EditorSaveCancelButton
            onSave={() => handleSave(validator)}
            onCancel={onCancel}
            disabled={
              attempt.status === 'processing' ||
              errors?.length > 0 ||
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
      label="Name of requested roles to match"
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
      value={ruleCondition?.rolesCondition?.values}
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
        Recipients can be Teams user emails/IDs, or a Teams channel URL, which
        can be obtained by opening the channel and selecting{' '}
        <Mark>Copy link</Mark>. Make sure to invite the bot to every
        team/channel.
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
