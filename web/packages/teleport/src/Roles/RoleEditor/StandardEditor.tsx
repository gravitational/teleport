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

import React, { useId, useState } from 'react';
import {
  Box,
  ButtonIcon,
  ButtonSecondary,
  Flex,
  H3,
  H4,
  Input,
  LabelInput,
  Mark,
  Text,
} from 'design';
import FieldInput from 'shared/components/FieldInput';
import { useValidation } from 'shared/components/Validation';
import {
  precomputed,
  ValidationResult,
} from 'shared/components/Validation/rules';
import * as Icon from 'design/Icon';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import styled, { useTheme } from 'styled-components';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { SlideTabs } from 'design/SlideTabs';

import { RadioGroup } from 'design/RadioGroup';

import Select from 'shared/components/Select';

import { Role, RoleWithYaml } from 'teleport/services/resources';
import { LabelsInput } from 'teleport/components/LabelsInput';

import { FieldMultiInput } from '../../../../shared/components/FieldMultiInput/FieldMultiInput';

import {
  roleEditorModelToRole,
  hasModifiedFields,
  MetadataModel,
  RoleEditorModel,
  StandardEditorModel,
  AccessSpecKind,
  AccessSpec,
  ServerAccessSpec,
  newAccessSpec,
  KubernetesAccessSpec,
  newKubernetesResourceModel,
  kubernetesResourceKindOptions,
  kubernetesVerbOptions,
  KubernetesResourceModel,
  AppAccessSpec,
  DatabaseAccessSpec,
  WindowsDesktopAccessSpec,
  RuleModel,
  resourceKindOptions,
  verbOptions,
  newRuleModel,
  OptionsModel,
  requireMFATypeOptions,
  createHostUserModeOptions,
} from './standardmodel';
import {
  validateRoleEditorModel,
  MetadataValidationResult,
  AccessSpecValidationResult,
  ServerSpecValidationResult,
  KubernetesSpecValidationResult,
  KubernetesResourceValidationResult,
  AppSpecValidationResult,
  DatabaseSpecValidationResult,
  WindowsDesktopSpecValidationResult,
  AccessRuleValidationResult,
} from './validation';
import { EditorSaveCancelButton } from './Shared';
import { RequiresResetToStandard } from './RequiresResetToStandard';

export type StandardEditorProps = {
  originalRole: RoleWithYaml;
  standardEditorModel: StandardEditorModel;
  isProcessing?: boolean;
  onSave?(r: Role): void;
  onCancel?(): void;
  onChange?(s: StandardEditorModel): void;
};

/**
 * A structured editor that represents a role with a series of UI controls, as
 * opposed to a YAML text editor.
 */
export const StandardEditor = ({
  originalRole,
  standardEditorModel,
  isProcessing,
  onSave,
  onCancel,
  onChange,
}: StandardEditorProps) => {
  const isEditing = !!originalRole;
  const { roleModel } = standardEditorModel;
  const validation = validateRoleEditorModel(roleModel);

  /** All spec kinds except those that are already in the role. */
  const allowedSpecKinds = allAccessSpecKinds.filter(k =>
    roleModel.accessSpecs.every(as => as.kind !== k)
  );

  enum StandardEditorTab {
    Overview,
    Resources,
    AccessRules,
    Options,
  }

  const [currentTab, setCurrentTab] = useState(StandardEditorTab.Overview);
  const idPrefix = useId();
  const overviewTabId = `${idPrefix}-overview`;
  const resourcesTabId = `${idPrefix}-resources`;
  const accessRulesTabId = `${idPrefix}-access-rules`;
  const optionsTabId = `${idPrefix}-options`;

  const validator = useValidation();

  function handleSave() {
    if (!validator.validate()) {
      return;
    }
    onSave?.(roleEditorModelToRole(standardEditorModel.roleModel));
  }

  function handleChange(modified: Partial<RoleEditorModel>) {
    const updatedResourceModel: RoleEditorModel = {
      ...roleModel,
      ...modified,
    };

    onChange?.({
      ...standardEditorModel,
      roleModel: updatedResourceModel,
      isDirty: hasModifiedFields(updatedResourceModel, originalRole?.object),
    });
  }

  function addAccessSpec(kind: AccessSpecKind) {
    handleChange({
      ...standardEditorModel.roleModel,
      accessSpecs: [
        ...standardEditorModel.roleModel.accessSpecs,
        newAccessSpec(kind),
      ],
    });
  }

  function removeAccessSpec(kind: AccessSpecKind) {
    handleChange({
      ...standardEditorModel.roleModel,
      accessSpecs: standardEditorModel.roleModel.accessSpecs.filter(
        s => s.kind !== kind
      ),
    });
  }

  function setAccessSpec(value: AccessSpec) {
    handleChange({
      ...standardEditorModel.roleModel,
      accessSpecs: standardEditorModel.roleModel.accessSpecs.map(original =>
        original.kind === value.kind ? value : original
      ),
    });
  }

  function setRules(rules: RuleModel[]) {
    handleChange({
      ...standardEditorModel.roleModel,
      rules,
    });
  }

  function setOptions(options: OptionsModel) {
    handleChange({
      ...standardEditorModel,
      options,
    });
  }

  return (
    <>
      {roleModel.requiresReset && (
        <Box mx={3}>
          <RequiresResetToStandard />
        </Box>
      )}
      <EditorWrapper
        mute={standardEditorModel.roleModel.requiresReset}
        data-testid="standard-editor"
      >
        <Box mb={3} mx={3}>
          <SlideTabs
            appearance="round"
            hideStatusIconOnActiveTab
            tabs={[
              {
                key: StandardEditorTab.Overview,
                title: 'Overview',
                controls: overviewTabId,
                status:
                  validator.state.validating && !validation.metadata.valid
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.Resources,
                title: 'Resources',
                controls: resourcesTabId,
                status:
                  validator.state.validating &&
                  validation.accessSpecs.some(s => !s.valid)
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.AccessRules,
                title: 'Access Rules',
                controls: accessRulesTabId,
                status:
                  validator.state.validating &&
                  validation.rules.some(s => !s.valid)
                    ? validationErrorTabStatus
                    : undefined,
              },
              {
                key: StandardEditorTab.Options,
                title: 'Options',
                controls: optionsTabId,
              },
            ]}
            activeIndex={currentTab}
            onChange={setCurrentTab}
          />
        </Box>
        <Flex
          flex="1 1 0"
          flexDirection="column"
          px={3}
          pb={3}
          css={`
            overflow-y: auto;
          `}
        >
          <Box
            id={overviewTabId}
            style={{
              display: currentTab === StandardEditorTab.Overview ? '' : 'none',
            }}
          >
            <MetadataSection
              value={roleModel.metadata}
              isProcessing={isProcessing}
              validation={validation.metadata}
              onChange={metadata => handleChange({ ...roleModel, metadata })}
            />
          </Box>
          <Box
            id={resourcesTabId}
            style={{
              display: currentTab === StandardEditorTab.Resources ? '' : 'none',
            }}
          >
            <Flex flexDirection="column" gap={3} my={2}>
              {roleModel.accessSpecs.map((spec, i) => {
                const validationResult = validation.accessSpecs[i];
                return (
                  <AccessSpecSection
                    key={spec.kind}
                    value={spec}
                    isProcessing={isProcessing}
                    validation={validationResult}
                    onChange={value => setAccessSpec(value)}
                    onRemove={() => removeAccessSpec(spec.kind)}
                  />
                );
              })}
              <Box>
                <MenuButton
                  menuProps={{
                    transformOrigin: {
                      vertical: 'bottom',
                      horizontal: 'right',
                    },
                    anchorOrigin: {
                      vertical: 'top',
                      horizontal: 'right',
                    },
                  }}
                  buttonText={
                    <>
                      <Icon.Plus size="small" mr={2} />
                      Add New Specifications
                    </>
                  }
                  buttonProps={{
                    size: 'medium',
                    fill: 'filled',
                    disabled: isProcessing || allowedSpecKinds.length === 0,
                  }}
                >
                  {allowedSpecKinds.map(kind => (
                    <MenuItem key={kind} onClick={() => addAccessSpec(kind)}>
                      {specSections[kind].title}
                    </MenuItem>
                  ))}
                </MenuButton>
              </Box>
            </Flex>
          </Box>
          <Box
            id={accessRulesTabId}
            style={{
              display:
                currentTab === StandardEditorTab.AccessRules ? '' : 'none',
            }}
          >
            <AccessRules
              isProcessing={isProcessing}
              value={roleModel.rules}
              onChange={setRules}
              validation={validation.rules}
            />
          </Box>
          <Box
            id={optionsTabId}
            style={{
              display: currentTab === StandardEditorTab.Options ? '' : 'none',
            }}
          >
            <Options
              isProcessing={isProcessing}
              value={roleModel.options}
              onChange={setOptions}
            />
          </Box>
        </Flex>
      </EditorWrapper>
      <EditorSaveCancelButton
        onSave={() => handleSave()}
        onCancel={onCancel}
        disabled={
          isProcessing ||
          standardEditorModel.roleModel.requiresReset ||
          !standardEditorModel.isDirty
        }
        isEditing={isEditing}
      />
    </>
  );
};

export type SectionProps<Model, ValidationResult> = {
  value: Model;
  isProcessing: boolean;
  validation?: ValidationResult;
  onChange?(value: Model): void;
};

const validationErrorTabStatus = {
  kind: 'danger',
  ariaLabel: 'Invalid data',
} as const;

const MetadataSection = ({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<MetadataModel, MetadataValidationResult>) => (
  <Section
    title="Role Metadata"
    tooltip="Basic information about the role resource"
    isProcessing={isProcessing}
    validation={validation}
  >
    <FieldInput
      label="Role Name"
      placeholder="Enter Role Name"
      value={value.name}
      disabled={isProcessing}
      rule={precomputed(validation.fields.name)}
      onChange={e => onChange({ ...value, name: e.target.value })}
    />
    <FieldInput
      label="Description"
      placeholder="Enter Role Description"
      value={value.description || ''}
      disabled={isProcessing}
      mb={0}
      onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
        onChange({ ...value, description: e.target.value })
      }
    />
  </Section>
);

/**
 * A wrapper for editor section. Its responsibility is rendering a header,
 * expanding, collapsing, and removing the section.
 */
const Section = ({
  title,
  tooltip,
  children,
  removable,
  isProcessing,
  validation,
  onRemove,
}: React.PropsWithChildren<{
  title: string;
  tooltip: string;
  removable?: boolean;
  isProcessing: boolean;
  validation?: ValidationResult;
  onRemove?(): void;
}>) => {
  const theme = useTheme();
  const [expanded, setExpanded] = useState(true);
  const ExpandIcon = expanded ? Icon.Minus : Icon.Plus;
  const expandTooltip = expanded ? 'Collapse' : 'Expand';
  const validator = useValidation();

  const handleExpand = (e: React.MouseEvent) => {
    // Don't let <summary> handle the event, we'll do it ourselves to keep
    // track of the state.
    e.preventDefault();
    setExpanded(expanded => !expanded);
  };

  const handleRemove = (e: React.MouseEvent) => {
    // Don't let <summary> handle the event.
    e.stopPropagation();
    onRemove?.();
  };

  return (
    <Box
      as="details"
      open={expanded}
      border={1}
      borderColor={
        validator.state.validating && !validation.valid
          ? theme.colors.interactive.solid.danger.default
          : theme.colors.interactive.tonal.neutral[0]
      }
      borderRadius={3}
    >
      <Flex
        as="summary"
        height="56px"
        alignItems="center"
        ml={3}
        mr={3}
        css={'cursor: pointer'}
        onClick={handleExpand}
      >
        {/* TODO(bl-nero): Show validation result in the summary. */}
        <Flex flex="1" gap={2}>
          <H3>{title}</H3>
          {tooltip && <IconTooltip>{tooltip}</IconTooltip>}
        </Flex>
        {removable && (
          <Box
            borderRight={1}
            borderColor={theme.colors.interactive.tonal.neutral[0]}
          >
            <HoverTooltip tipContent="Remove section">
              <ButtonIcon
                aria-label="Remove section"
                disabled={isProcessing}
                onClick={handleRemove}
              >
                <Icon.Trash
                  size="small"
                  color={theme.colors.interactive.solid.danger.default}
                />
              </ButtonIcon>
            </HoverTooltip>
          </Box>
        )}
        <HoverTooltip tipContent={expandTooltip}>
          <ExpandIcon size="small" color={theme.colors.text.muted} ml={2} />
        </HoverTooltip>
      </Flex>
      <Box px={3} pb={3}>
        {children}
      </Box>
    </Box>
  );
};

/**
 * All access spec kinds, in order of appearance in the resource kind dropdown.
 */
const allAccessSpecKinds: AccessSpecKind[] = [
  'kube_cluster',
  'node',
  'app',
  'db',
  'windows_desktop',
];

/** Maps access specification kind to UI component configuration. */
const specSections: Record<
  AccessSpecKind,
  {
    title: string;
    tooltip: string;
    component: React.ComponentType<SectionProps<unknown, unknown>>;
  }
> = {
  kube_cluster: {
    title: 'Kubernetes',
    tooltip: 'Configures access to Kubernetes clusters',
    component: KubernetesAccessSpecSection,
  },
  node: {
    title: 'Servers',
    tooltip: 'Configures access to SSH servers',
    component: ServerAccessSpecSection,
  },
  app: {
    title: 'Applications',
    tooltip: 'Configures access to applications',
    component: AppAccessSpecSection,
  },
  db: {
    title: 'Databases',
    tooltip: 'Configures access to databases',
    component: DatabaseAccessSpecSection,
  },
  windows_desktop: {
    title: 'Windows Desktops',
    tooltip: 'Configures access to Windows desktops',
    component: WindowsDesktopAccessSpecSection,
  },
};

/**
 * A generic access spec section. Details are rendered by components from the
 * `specSections` map.
 */
const AccessSpecSection = <
  T extends AccessSpec,
  V extends AccessSpecValidationResult,
>({
  value,
  isProcessing,
  validation,
  onChange,
  onRemove,
}: SectionProps<T, V> & {
  onRemove?(): void;
}) => {
  const { component: Body, title, tooltip } = specSections[value.kind];
  return (
    <Section
      title={title}
      removable
      onRemove={onRemove}
      tooltip={tooltip}
      isProcessing={isProcessing}
      validation={validation}
    >
      <Body
        value={value}
        isProcessing={isProcessing}
        validation={validation}
        onChange={onChange}
      />
    </Section>
  );
};

export function ServerAccessSpecSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<ServerAccessSpec, ServerSpecValidationResult>) {
  return (
    <>
      <Text typography="body3" mb={1}>
        Labels
      </Text>
      <LabelsInput
        disableBtns={isProcessing}
        labels={value.labels}
        setLabels={labels => onChange?.({ ...value, labels })}
        rule={precomputed(validation.fields.labels)}
      />
      <FieldSelectCreatable
        isMulti
        label="Logins"
        isDisabled={isProcessing}
        formatCreateLabel={label => `Login: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.logins}
        onChange={logins => onChange?.({ ...value, logins })}
        rule={precomputed(validation.fields.logins)}
        mt={3}
        mb={0}
      />
    </>
  );
}

export function KubernetesAccessSpecSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<KubernetesAccessSpec, KubernetesSpecValidationResult>) {
  return (
    <>
      <FieldSelectCreatable
        isMulti
        label="Groups"
        isDisabled={isProcessing}
        formatCreateLabel={label => `Group: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.groups}
        onChange={groups => onChange?.({ ...value, groups })}
      />

      <Text typography="body3" mb={1}>
        Labels
      </Text>
      <LabelsInput
        disableBtns={isProcessing}
        labels={value.labels}
        rule={precomputed(validation.fields.labels)}
        setLabels={labels => onChange?.({ ...value, labels })}
      />

      <Flex flexDirection="column" gap={3} mt={3}>
        {value.resources.map((resource, index) => (
          <KubernetesResourceView
            key={resource.id}
            value={resource}
            validation={validation.fields.resources.results[index]}
            isProcessing={isProcessing}
            onChange={newRes =>
              onChange?.({
                ...value,
                resources: value.resources.map((res, i) =>
                  i === index ? newRes : res
                ),
              })
            }
            onRemove={() =>
              onChange?.({
                ...value,
                resources: value.resources.toSpliced(index, 1),
              })
            }
          />
        ))}

        <Box>
          <ButtonSecondary
            disabled={isProcessing}
            gap={1}
            onClick={() =>
              onChange?.({
                ...value,
                resources: [...value.resources, newKubernetesResourceModel()],
              })
            }
          >
            <Icon.Add disabled={isProcessing} size="small" />
            {value.resources.length > 0
              ? 'Add Another Resource'
              : 'Add a Resource'}
          </ButtonSecondary>
        </Box>
      </Flex>
    </>
  );
}

function KubernetesResourceView({
  value,
  validation,
  isProcessing,
  onChange,
  onRemove,
}: {
  value: KubernetesResourceModel;
  validation: KubernetesResourceValidationResult;
  isProcessing: boolean;
  onChange(m: KubernetesResourceModel): void;
  onRemove(): void;
}) {
  const { kind, name, namespace, verbs } = value;
  const theme = useTheme();
  return (
    <Box
      border={1}
      borderColor={theme.colors.interactive.tonal.neutral[0]}
      borderRadius={3}
      padding={3}
    >
      <Flex>
        <Box flex="1">
          <H4 mb={3}>Resource</H4>
        </Box>
        <ButtonIcon
          aria-label="Remove resource"
          disabled={isProcessing}
          onClick={onRemove}
        >
          <Icon.Trash
            size="small"
            color={theme.colors.interactive.solid.danger.default}
          />
        </ButtonIcon>
      </Flex>
      <FieldSelect
        label="Kind"
        isDisabled={isProcessing}
        options={kubernetesResourceKindOptions}
        value={kind}
        onChange={k => onChange?.({ ...value, kind: k })}
      />
      <FieldInput
        label="Name"
        toolTipContent={
          <>
            Name of the resource. Special value <MarkInverse>*</MarkInverse>{' '}
            means any name.
          </>
        }
        disabled={isProcessing}
        value={name}
        rule={precomputed(validation.name)}
        onChange={e => onChange?.({ ...value, name: e.target.value })}
      />
      <FieldInput
        label="Namespace"
        toolTipContent={
          <>
            Namespace that contains the resource. Special value{' '}
            <MarkInverse>*</MarkInverse> means any namespace.
          </>
        }
        disabled={isProcessing}
        value={namespace}
        rule={precomputed(validation.namespace)}
        onChange={e => onChange?.({ ...value, namespace: e.target.value })}
      />
      <FieldSelect
        isMulti
        label="Verbs"
        isDisabled={isProcessing}
        options={kubernetesVerbOptions}
        value={verbs}
        onChange={v => onChange?.({ ...value, verbs: v })}
        mb={0}
      />
    </Box>
  );
}

export function AppAccessSpecSection({
  value,
  validation,
  isProcessing,
  onChange,
}: SectionProps<AppAccessSpec, AppSpecValidationResult>) {
  return (
    <Flex flexDirection="column" gap={3}>
      <Box>
        <Text typography="body3" mb={1}>
          Labels
        </Text>
        <LabelsInput
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange?.({ ...value, labels })}
          rule={precomputed(validation.fields.labels)}
        />
      </Box>
      <FieldMultiInput
        label="AWS Role ARNs"
        disabled={isProcessing}
        value={value.awsRoleARNs}
        onChange={arns => onChange?.({ ...value, awsRoleARNs: arns })}
        rule={precomputed(validation.fields.awsRoleARNs)}
      />
      <FieldMultiInput
        label="Azure Identities"
        disabled={isProcessing}
        value={value.azureIdentities}
        onChange={ids => onChange?.({ ...value, azureIdentities: ids })}
        rule={precomputed(validation.fields.azureIdentities)}
      />
      <FieldMultiInput
        label="GCP Service Accounts"
        disabled={isProcessing}
        value={value.gcpServiceAccounts}
        onChange={accts => onChange?.({ ...value, gcpServiceAccounts: accts })}
        rule={precomputed(validation.fields.gcpServiceAccounts)}
      />
    </Flex>
  );
}

export function DatabaseAccessSpecSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<DatabaseAccessSpec, DatabaseSpecValidationResult>) {
  return (
    <>
      <Box mb={3}>
        <Text typography="body3" mb={1}>
          Labels
        </Text>
        <LabelsInput
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange?.({ ...value, labels })}
          rule={precomputed(validation.fields.labels)}
        />
      </Box>
      <FieldSelectCreatable
        isMulti
        label="Database Names"
        toolTipContent={
          <>
            List of database names that this role is allowed to connect to.
            Special value <MarkInverse>*</MarkInverse> means any name.
          </>
        }
        isDisabled={isProcessing}
        formatCreateLabel={label => `Database Name: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.names}
        onChange={names => onChange?.({ ...value, names })}
      />
      <FieldSelectCreatable
        isMulti
        label="Database Users"
        toolTipContent={
          <>
            List of database users that this role is allowed to connect as.
            Special value <MarkInverse>*</MarkInverse> means any user.
          </>
        }
        isDisabled={isProcessing}
        formatCreateLabel={label => `Database User: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.users}
        onChange={users => onChange?.({ ...value, users })}
      />
      <FieldSelectCreatable
        isMulti
        label="Database Roles"
        toolTipContent="If automatic user provisioning is available, this is the list of database roles that will be assigned to the database user after it's created"
        isDisabled={isProcessing}
        formatCreateLabel={label => `Database Role: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.roles}
        onChange={roles => onChange?.({ ...value, roles })}
        rule={precomputed(validation.fields.roles)}
        mb={0}
      />
    </>
  );
}

export function WindowsDesktopAccessSpecSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<WindowsDesktopAccessSpec, WindowsDesktopSpecValidationResult>) {
  return (
    <>
      <Box mb={3}>
        <Text typography="body3" mb={1}>
          Labels
        </Text>
        <LabelsInput
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange?.({ ...value, labels })}
          rule={precomputed(validation.fields.labels)}
        />
      </Box>
      <FieldSelectCreatable
        isMulti
        label="Logins"
        toolTipContent="List of desktop logins that this role is allowed to use"
        isDisabled={isProcessing}
        formatCreateLabel={label => `Login: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.logins}
        onChange={logins => onChange?.({ ...value, logins })}
      />
    </>
  );
}

export function AccessRules({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<RuleModel[], AccessRuleValidationResult[]>) {
  function addRule() {
    onChange?.([...value, newRuleModel()]);
  }
  function setRule(rule: RuleModel) {
    onChange?.(value.map(r => (r.id === rule.id ? rule : r)));
  }
  function removeRule(id: string) {
    onChange?.(value.filter(r => r.id !== id));
  }
  return (
    <Flex flexDirection="column" gap={3}>
      {value.map((rule, i) => (
        <AccessRule
          key={rule.id}
          isProcessing={isProcessing}
          value={rule}
          onChange={setRule}
          validation={validation[i]}
          onRemove={() => removeRule(rule.id)}
        />
      ))}
      <ButtonSecondary alignSelf="start" onClick={addRule}>
        <Icon.Plus size="small" mr={2} />
        Add New
      </ButtonSecondary>
    </Flex>
  );
}

function AccessRule({
  value,
  isProcessing,
  validation,
  onChange,
  onRemove,
}: SectionProps<RuleModel, AccessRuleValidationResult> & {
  onRemove?(): void;
}) {
  const { resources, verbs } = value;
  return (
    <Section
      title="Access Rule"
      tooltip="A rule that gives users access to certain kinds of resources"
      removable
      isProcessing={isProcessing}
      validation={validation}
      onRemove={onRemove}
    >
      <FieldSelect
        isMulti
        label="Resources"
        isDisabled={isProcessing}
        options={resourceKindOptions}
        value={resources}
        onChange={r => onChange?.({ ...value, resources: r })}
        rule={precomputed(validation.fields.resources)}
      />
      <FieldSelect
        isMulti
        label="Permissions"
        isDisabled={isProcessing}
        options={verbOptions}
        value={verbs}
        onChange={v => onChange?.({ ...value, verbs: v })}
        rule={precomputed(validation.fields.verbs)}
        mb={0}
      />
    </Section>
  );
}

function Options({
  value,
  isProcessing,
  onChange,
}: SectionProps<OptionsModel, never>) {
  const theme = useTheme();
  const id = useId();
  const maxSessionTTLId = `${id}-max-session-ttl`;
  const clientIdleTimeoutId = `${id}-client-idle-timeout`;
  const requireMFATypeId = `${id}-require-mfa-type`;
  const createHostUserModeId = `${id}-create-host-user-mode`;
  return (
    <OptionsGridContainer
      border={1}
      borderColor={theme.colors.interactive.tonal.neutral[0]}
      borderRadius={3}
      p={3}
    >
      <OptionsHeader>Global Settings</OptionsHeader>

      <OptionLabel htmlFor={maxSessionTTLId}>Max Session TTL</OptionLabel>
      <Input
        id={maxSessionTTLId}
        value={value.maxSessionTTL}
        disabled={isProcessing}
        onChange={e => onChange({ ...value, maxSessionTTL: e.target.value })}
      />

      <OptionLabel htmlFor={clientIdleTimeoutId}>
        Client Idle Timeout
      </OptionLabel>
      <Input
        id={clientIdleTimeoutId}
        value={value.clientIdleTimeout}
        disabled={isProcessing}
        onChange={e =>
          onChange({ ...value, clientIdleTimeout: e.target.value })
        }
      />

      <Box>Disconnect When Certificate Expires</Box>
      <BoolRadioGroup
        name="disconnect-expired-cert"
        value={value.disconnectExpiredCert}
        onChange={d => onChange({ ...value, disconnectExpiredCert: d })}
      />

      <OptionLabel htmlFor={requireMFATypeId}>Require Session MFA</OptionLabel>
      <Select
        inputId={requireMFATypeId}
        isDisabled={isProcessing}
        options={requireMFATypeOptions}
        value={value.requireMFAType}
        onChange={t => onChange?.({ ...value, requireMFAType: t })}
      />

      <OptionsHeader separator>SSH</OptionsHeader>

      <OptionLabel htmlFor={createHostUserModeId}>
        Create Host User Mode
      </OptionLabel>
      <Select
        inputId={createHostUserModeId}
        isDisabled={isProcessing}
        options={createHostUserModeOptions}
        value={value.createHostUserMode}
        onChange={m => onChange?.({ ...value, createHostUserMode: m })}
      />

      <OptionsHeader separator>Database</OptionsHeader>

      <Box>Create Database User</Box>
      <BoolRadioGroup
        name="create-db-user"
        value={value.createDBUser}
        onChange={c => onChange({ ...value, createDBUser: c })}
      />

      {/* TODO(bl-nero): a bug in YAML unmarshalling backend breaks the
          createDBUserMode field. Fix it and add the field here. */}

      <OptionsHeader separator>Desktop</OptionsHeader>

      <Box>Create Desktop User</Box>
      <BoolRadioGroup
        name="create-desktop-user"
        value={value.createDesktopUser}
        onChange={c => onChange({ ...value, createDesktopUser: c })}
      />

      <Box>Allow Clipboard Sharing</Box>
      <BoolRadioGroup
        name="desktop-clipboard"
        value={value.desktopClipboard}
        onChange={c => onChange({ ...value, desktopClipboard: c })}
      />

      <Box>Allow Directory Sharing</Box>
      <BoolRadioGroup
        name="desktop-directory-sharing"
        value={value.desktopDirectorySharing}
        onChange={s => onChange({ ...value, desktopDirectorySharing: s })}
      />
    </OptionsGridContainer>
  );
}

const OptionsGridContainer = styled(Box)`
  display: grid;
  grid-template-columns: 1fr 1fr;
  align-items: baseline;
  row-gap: ${props => props.theme.space[3]}px;
`;

const OptionsHeader = styled(H4)<{ separator?: boolean }>`
  grid-column: 1/3;
  border-top: ${props =>
    props.separator
      ? `${props.theme.borders[1]} ${props.theme.colors.interactive.tonal.neutral[0]}`
      : 'none'};
  padding-top: ${props =>
    props.separator ? `${props.theme.space[3]}px` : '0'};
`;

function BoolRadioGroup({
  name,
  value,
  onChange,
}: {
  name: string;
  value: boolean;
  onChange(b: boolean): void;
}) {
  return (
    <RadioGroup
      name={name}
      flexDirection="row"
      options={[
        { label: 'True', value: 'true' },
        { label: 'False', value: 'false' },
      ]}
      value={String(value)}
      onChange={d => onChange(d === 'true')}
    />
  );
}

const OptionLabel = styled(LabelInput)`
  ${props => props.theme.typography.body2}
`;

export const EditorWrapper = styled(Flex)<{ mute?: boolean }>`
  flex-direction: column;
  flex: 1;
  opacity: ${p => (p.mute ? 0.4 : 1)};
  pointer-events: ${p => (p.mute ? 'none' : '')};
`;

// TODO(bl-nero): This should ideally use tonal neutral 1 from the opposite
// theme as background.
const MarkInverse = styled(Mark)`
  background: ${p => p.theme.colors.text.primaryInverse};
  color: ${p => p.theme.colors.text.main};
`;
