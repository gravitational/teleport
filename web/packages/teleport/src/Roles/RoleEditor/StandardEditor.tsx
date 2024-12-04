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

import React, { useState } from 'react';
import {
  Box,
  ButtonIcon,
  ButtonSecondary,
  Flex,
  H3,
  H4,
  Mark,
  Text,
} from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import * as Icon from 'design/Icon';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import styled, { useTheme } from 'styled-components';

import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';

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
} from './standardmodel';
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

  /** All spec kinds except those that are already in the role. */
  const allowedSpecKinds = allAccessSpecKinds.filter(k =>
    roleModel.accessSpecs.every(as => as.kind !== k)
  );

  function handleSave(validator: Validator) {
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

  /**
   * Resets the standard editor back into viewable state. The existing model
   * has been already stripped from unsupported features by the parsing
   * attempt, the only thing left to do is to set the `requiresReset` flag.
   */
  function resetForStandardEditor() {
    handleChange({
      ...standardEditorModel.roleModel,
      requiresReset: false,
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

  return (
    <Validation>
      {({ validator }) => (
        <>
          {roleModel.requiresReset && (
            <RequiresResetToStandard reset={resetForStandardEditor} />
          )}
          <EditorWrapper
            mute={standardEditorModel.roleModel.requiresReset}
            data-testid="standard-editor"
          >
            <Flex flexDirection="column" gap={3} my={2}>
              <MetadataSection
                value={roleModel.metadata}
                isProcessing={isProcessing}
                onChange={metadata => handleChange({ ...roleModel, metadata })}
              />
              {roleModel.accessSpecs.map(spec => (
                <AccessSpecSection
                  key={spec.kind}
                  value={spec}
                  isProcessing={isProcessing}
                  onChange={value => setAccessSpec(value)}
                  onRemove={() => removeAccessSpec(spec.kind)}
                />
              ))}
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
          </EditorWrapper>
          <EditorSaveCancelButton
            onSave={() => handleSave(validator)}
            onCancel={onCancel}
            disabled={
              isProcessing ||
              standardEditorModel.roleModel.requiresReset ||
              !standardEditorModel.isDirty
            }
            isEditing={isEditing}
          />
        </>
      )}
    </Validation>
  );
};

export type SectionProps<T> = {
  value: T;
  isProcessing: boolean;
  onChange?(value: T): void;
};

const MetadataSection = ({
  value,
  isProcessing,
  onChange,
}: SectionProps<MetadataModel>) => (
  <Section
    title="Role Metadata"
    tooltip="Basic information about the role resource"
    isProcessing={isProcessing}
  >
    <FieldInput
      label="Role Name"
      placeholder="Enter Role Name"
      value={value.name}
      disabled={isProcessing}
      rule={requiredField('Role name is required')}
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
  onRemove,
}: React.PropsWithChildren<{
  title: string;
  tooltip: string;
  removable?: boolean;
  isProcessing: boolean;
  onRemove?(): void;
}>) => {
  const theme = useTheme();
  const [expanded, setExpanded] = useState(true);
  const ExpandIcon = expanded ? Icon.Minus : Icon.Plus;
  const expandTooltip = expanded ? 'Collapse' : 'Expand';

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
      borderColor={theme.colors.interactive.tonal.neutral[0]}
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
    component: React.ComponentType<SectionProps<unknown>>;
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
const AccessSpecSection = <T extends AccessSpec>({
  value,
  isProcessing,
  onChange,
  onRemove,
}: SectionProps<T> & {
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
    >
      <Body value={value} isProcessing={isProcessing} onChange={onChange} />
    </Section>
  );
};

export function ServerAccessSpecSection({
  value,
  isProcessing,
  onChange,
}: SectionProps<ServerAccessSpec>) {
  return (
    <>
      <Text typography="body3" mb={1}>
        Labels
      </Text>
      <LabelsInput
        disableBtns={isProcessing}
        labels={value.labels}
        setLabels={labels => onChange?.({ ...value, labels })}
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
        mt={3}
        mb={0}
      />
    </>
  );
}

export function KubernetesAccessSpecSection({
  value,
  isProcessing,
  onChange,
}: SectionProps<KubernetesAccessSpec>) {
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
        setLabels={labels => onChange?.({ ...value, labels })}
      />

      <Flex flexDirection="column" gap={3} mt={3}>
        {value.resources.map((resource, index) => (
          <KubernetesResourceView
            key={resource.id}
            value={resource}
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
  isProcessing,
  onChange,
  onRemove,
}: {
  value: KubernetesResourceModel;
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
  isProcessing,
  onChange,
}: SectionProps<AppAccessSpec>) {
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
        />
      </Box>
      <FieldMultiInput
        label="AWS Role ARNs"
        disabled={isProcessing}
        value={value.awsRoleARNs}
        onChange={arns => onChange?.({ ...value, awsRoleARNs: arns })}
      />
      <FieldMultiInput
        label="Azure Identities"
        disabled={isProcessing}
        value={value.azureIdentities}
        onChange={ids => onChange?.({ ...value, azureIdentities: ids })}
      />
      <FieldMultiInput
        label="GCP Service Accounts"
        disabled={isProcessing}
        value={value.gcpServiceAccounts}
        onChange={accts => onChange?.({ ...value, gcpServiceAccounts: accts })}
      />
    </Flex>
  );
}

export function DatabaseAccessSpecSection({
  value,
  isProcessing,
  onChange,
}: SectionProps<DatabaseAccessSpec>) {
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
        mb={0}
      />
    </>
  );
}

export function WindowsDesktopAccessSpecSection({
  value,
  isProcessing,
  onChange,
}: SectionProps<WindowsDesktopAccessSpec>) {
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

export const EditorWrapper = styled(Box)<{ mute?: boolean }>`
  opacity: ${p => (p.mute ? 0.4 : 1)};
  pointer-events: ${p => (p.mute ? 'none' : '')};
`;

// TODO(bl-nero): This should ideally use tonal neutral 1 from the opposite
// theme as background.
const MarkInverse = styled(Mark)`
  background: ${p => p.theme.colors.text.primaryInverse};
  color: ${p => p.theme.colors.text.main};
`;
