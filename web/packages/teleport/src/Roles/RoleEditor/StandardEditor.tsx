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
import { Box, ButtonIcon, Flex, H3, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import * as Icon from 'design/Icon';
import { HoverTooltip, ToolTipInfo } from 'shared/components/ToolTip';
import styled, { useTheme } from 'styled-components';

import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import { FieldSelectCreatable } from 'shared/components/FieldSelect';

import { Role, RoleWithYaml } from 'teleport/services/resources';

import { LabelsInput } from 'teleport/components/LabelsInput';

import {
  roleEditorModelToRole,
  hasModifiedFields,
  MetadataModel,
  RoleEditorModel,
  StandardEditorModel,
  AccessSpecKind,
  AccessSpec,
  ServerAccessSpec,
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
        { kind, labels: [], logins: [] },
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

type SectionProps<T> = {
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
      borderRadius={2}
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
          {tooltip && <ToolTipInfo>{tooltip}</ToolTipInfo>}
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
const allAccessSpecKinds: AccessSpecKind[] = ['kube_cluster', 'node'];

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
        value={value.logins}
        onChange={logins => onChange?.({ ...value, logins })}
        mt={3}
        mb={0}
      />
    </>
  );
}

function KubernetesAccessSpecSection() {
  // TODO(bl-nero): add the Kubernetes section
  return null;
}

export const EditorWrapper = styled(Box)<{ mute?: boolean }>`
  opacity: ${p => (p.mute ? 0.4 : 1)};
  pointer-events: ${p => (p.mute ? 'none' : '')};
`;
