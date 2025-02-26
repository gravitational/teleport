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

import { memo } from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import { Add, Plus, Trash } from 'design/Icon';
import { Mark } from 'design/Mark';
import { H4 } from 'design/Text';
import FieldInput from 'shared/components/FieldInput';
import { FieldMultiInput } from 'shared/components/FieldMultiInput/FieldMultiInput';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { precomputed } from 'shared/components/Validation/rules';

import { LabelsInput } from 'teleport/components/LabelsInput';

import { SectionBox, SectionProps, SectionPropsWithDispatch } from './sections';
import {
  AppAccess,
  DatabaseAccess,
  GitHubOrganizationAccess,
  KubernetesAccess,
  kubernetesResourceKindOptions,
  KubernetesResourceModel,
  kubernetesVerbOptions,
  newKubernetesResourceModel,
  ResourceAccess,
  ResourceAccessKind,
  ServerAccess,
  WindowsDesktopAccess,
} from './standardmodel';
import {
  AppAccessValidationResult,
  DatabaseAccessValidationResult,
  GitHubOrganizationAccessValidationResult,
  KubernetesAccessValidationResult,
  KubernetesResourceValidationResult,
  ResourceAccessValidationResult,
  ServerAccessValidationResult,
  WindowsDesktopAccessValidationResult,
} from './validation';

/**
 * Resources tab. This component is memoized to optimize performance; make sure
 * that the properties don't change unless necessary.
 */
export const ResourcesTab = memo(function ResourcesTab({
  value,
  isProcessing,
  validation,
  dispatch,
}: SectionPropsWithDispatch<
  ResourceAccess[],
  ResourceAccessValidationResult[]
>) {
  /** All resource access kinds except those that are already in the role. */
  const allowedResourceAccessKinds = allResourceAccessKinds.filter(k =>
    value.every(as => as.kind !== k)
  );

  const addResourceAccess = (kind: ResourceAccessKind) =>
    dispatch({ type: 'add-resource-access', payload: { kind } });

  return (
    <Flex flexDirection="column" gap={3} my={2}>
      {value.map((res, i) => {
        return (
          <ResourceAccessSection
            key={res.kind}
            value={res}
            isProcessing={isProcessing}
            validation={validation[i]}
            dispatch={dispatch}
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
              <Plus size="small" mr={2} />
              Add New Resource Access
            </>
          }
          buttonProps={{
            size: 'medium',
            fill: 'filled',
            disabled: isProcessing || allowedResourceAccessKinds.length === 0,
          }}
        >
          {allowedResourceAccessKinds.map(kind => (
            <MenuItem key={kind} onClick={() => addResourceAccess(kind)}>
              {resourceAccessSections[kind].title}
            </MenuItem>
          ))}
        </MenuButton>
      </Box>
    </Flex>
  );
});

/**
 * All resource access kinds, in order of appearance in the resource kind
 * dropdown.
 */
const allResourceAccessKinds: ResourceAccessKind[] = [
  'kube_cluster',
  'node',
  'app',
  'db',
  'windows_desktop',
  'git_server',
];

/** Maps resource access kind to UI component configuration. */
export const resourceAccessSections: Record<
  ResourceAccessKind,
  {
    title: string;
    tooltip: string;
    component: React.ComponentType<SectionProps<unknown, unknown>>;
  }
> = {
  kube_cluster: {
    title: 'Kubernetes',
    tooltip: 'Configures access to Kubernetes clusters',
    component: KubernetesAccessSection,
  },
  node: {
    title: 'Servers',
    tooltip: 'Configures access to SSH servers',
    component: ServerAccessSection,
  },
  app: {
    title: 'Applications',
    tooltip: 'Configures access to applications',
    component: AppAccessSection,
  },
  db: {
    title: 'Databases',
    tooltip: 'Configures access to databases',
    component: DatabaseAccessSection,
  },
  windows_desktop: {
    title: 'Windows Desktops',
    tooltip: 'Configures access to Windows desktops',
    component: WindowsDesktopAccessSection,
  },
  git_server: {
    title: 'GitHub Organizations',
    tooltip: 'Configures access to GitHub organizations and their repositories',
    component: GitHubOrganizationAccessSection,
  },
};

/**
 * A generic resource section. Details are rendered by components from the
 * `resourceAccessSections` map.
 */
export const ResourceAccessSection = memo(function ResourceAccessSectionRaw<
  T extends ResourceAccess,
  V extends ResourceAccessValidationResult,
>({
  value,
  isProcessing,
  validation,
  dispatch,
}: SectionPropsWithDispatch<T, V>) {
  const {
    component: Body,
    title,
    tooltip,
  } = resourceAccessSections[value.kind];

  function handleChange(val: T) {
    dispatch({ type: 'set-resource-access', payload: val });
  }

  function handleRemove() {
    dispatch({ type: 'remove-resource-access', payload: { kind: value.kind } });
  }

  return (
    <SectionBox
      title={title}
      removable
      onRemove={handleRemove}
      tooltip={tooltip}
      isProcessing={isProcessing}
      validation={validation}
    >
      <Body
        value={value}
        isProcessing={isProcessing}
        validation={validation}
        onChange={handleChange}
      />
    </SectionBox>
  );
});

export function ServerAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<ServerAccess, ServerAccessValidationResult>) {
  return (
    <>
      <LabelsInput
        legend="Labels"
        disableBtns={isProcessing}
        labels={value.labels}
        setLabels={labels => onChange?.({ ...value, labels })}
        rule={precomputed(validation.fields.labels)}
      />
      <FieldSelectCreatable
        isMulti
        label="Logins"
        placeholder="Type a login and press Enter"
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

export function KubernetesAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<KubernetesAccess, KubernetesAccessValidationResult>) {
  return (
    <>
      <FieldSelectCreatable
        isMulti
        label="Groups"
        placeholder="Type a group name and press Enter"
        isDisabled={isProcessing}
        formatCreateLabel={label => `Group: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.groups}
        onChange={groups => onChange?.({ ...value, groups })}
      />

      <FieldSelectCreatable
        isMulti
        label="Users"
        placeholder="Type a user name and press Enter"
        isDisabled={isProcessing}
        formatCreateLabel={label => `User: ${label}`}
        components={{
          DropdownIndicator: null,
        }}
        openMenuOnClick={false}
        value={value.users}
        onChange={users => onChange?.({ ...value, users })}
      />

      <LabelsInput
        legend="Labels"
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
                resources: [
                  ...value.resources,
                  newKubernetesResourceModel(value.roleVersion),
                ],
              })
            }
          >
            <Add disabled={isProcessing} size="small" />
            {value.resources.length > 0
              ? 'Add Another Resource'
              : 'Add a Resource'}
          </ButtonSecondary>
        </Box>
      </Flex>
    </>
  );
}

function KubernetesResourceKindView({
  value,
  validation,
  isProcessing,
  onChange,
}: {
  value: KubernetesResourceModel;
  validation: KubernetesResourceValidationResult['kind'];
  isProcessing: boolean;
  onChange?(m: KubernetesResourceModel): void;
}) {
  const selectField = (
    <FieldSelect
      label="Kind"
      isDisabled={isProcessing}
      options={kubernetesResourceKindOptions}
      value={value.kind}
      rule={precomputed(validation)}
      onChange={k => onChange?.({ ...value, kind: k })}
    />
  );
  if (value.kind.label !== 'CustomResource') {
    // If we are not dealing with a custom resource, return the select field.
    return selectField;
  }
  // If we are dealing with a custom resource, show an input field.
  return (
    <>
      {selectField}
      <FieldInput
        label="Custom Resource full name including group/version"
        disabled={isProcessing}
        value={value.kind.value}
        placeholder='e.g. "stable.example.com/v1/MyResource"'
        rule={precomputed(validation)}
        onChange={e =>
          onChange?.({
            ...value,
            kind: { ...value.kind, value: e.target.value },
          })
        }
      />
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
  const { name, namespace, verbs } = value;
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
          <Trash
            size="small"
            color={theme.colors.interactive.solid.danger.default}
          />
        </ButtonIcon>
      </Flex>
      <KubernetesResourceKindView
        value={value}
        validation={validation.kind}
        isProcessing={isProcessing}
        onChange={onChange}
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
        rule={precomputed(validation.verbs)}
        onChange={v => onChange?.({ ...value, verbs: v })}
        mb={0}
      />
    </Box>
  );
}

export function AppAccessSection({
  value,
  validation,
  isProcessing,
  onChange,
}: SectionProps<AppAccess, AppAccessValidationResult>) {
  return (
    <Flex flexDirection="column" gap={3}>
      <LabelsInput
        legend="Labels"
        disableBtns={isProcessing}
        labels={value.labels}
        setLabels={labels => onChange?.({ ...value, labels })}
        rule={precomputed(validation.fields.labels)}
      />
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

export function DatabaseAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<DatabaseAccess, DatabaseAccessValidationResult>) {
  return (
    <>
      <Box mb={3}>
        <LabelsInput
          legend="Labels"
          tooltipContent="Access to databases with these labels will be affected by this role"
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange?.({ ...value, labels })}
          rule={precomputed(validation.fields.labels)}
        />
      </Box>
      <FieldSelectCreatable
        isMulti
        label="Database Names"
        placeholder="Type a database name and press Enter"
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
        placeholder="Type a user name and press Enter"
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
        placeholder="Type a role name and press Enter"
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
      />
      <LabelsInput
        legend="Database Service Labels"
        tooltipContent="The database service labels control which Database Services (Teleport Agents) are visible to the user, which is required when adding Databases in the Enroll New Resource wizard. Access to Databases themselves is controlled by the Database Labels field."
        disableBtns={isProcessing}
        labels={value.dbServiceLabels}
        setLabels={dbServiceLabels => onChange?.({ ...value, dbServiceLabels })}
        rule={precomputed(validation.fields.dbServiceLabels)}
      />
    </>
  );
}

export function WindowsDesktopAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<WindowsDesktopAccess, WindowsDesktopAccessValidationResult>) {
  return (
    <>
      <Box mb={3}>
        <LabelsInput
          legend="Labels"
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange?.({ ...value, labels })}
          rule={precomputed(validation.fields.labels)}
        />
      </Box>
      <FieldSelectCreatable
        isMulti
        label="Logins"
        placeholder="Type a login and press Enter"
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

export function GitHubOrganizationAccessSection({
  value,
  isProcessing,
  onChange,
}: SectionProps<
  GitHubOrganizationAccess,
  GitHubOrganizationAccessValidationResult
>) {
  return (
    <FieldSelectCreatable
      isMulti
      label="Organization Names"
      toolTipContent="A list of GitHub organization names that this role is allowed to use"
      placeholder="Type an organization name and press Enter"
      isDisabled={isProcessing}
      formatCreateLabel={label => `Organization: ${label}`}
      components={{
        DropdownIndicator: null,
      }}
      openMenuOnClick={false}
      value={value.organizations}
      onChange={organizations => onChange?.({ ...value, organizations })}
    />
  );
}

// TODO(bl-nero): This should ideally use tonal neutral 1 from the opposite
// theme as background.
const MarkInverse = styled(Mark)`
  background: ${p => p.theme.colors.text.primaryInverse};
  color: ${p => p.theme.colors.text.main};
`;
