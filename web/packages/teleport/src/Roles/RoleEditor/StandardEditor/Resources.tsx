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
import { Button } from 'design/Button';
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
import { defaultRule } from 'shared/components/FieldSelect/shared';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { useRule } from 'shared/components/Validation';
import { precomputed } from 'shared/components/Validation/rules';
import { ValidationSuspender } from 'shared/components/Validation/Validation';

import { LabelsInput } from 'teleport/components/LabelsInput';
import { RoleVersion } from 'teleport/services/resources';

import {
  SectionBox,
  SectionPadding,
  SectionProps,
  SectionPropsWithDispatch,
} from './sections';
import {
  AppAccess,
  AppAccessInputFields,
  DatabaseAccess,
  DatabaseAccessInputFields,
  GitHubOrganizationAccess,
  GitHubOrganizationAccessInputFields,
  KubernetesAccess,
  KubernetesAccessInputFields,
  kubernetesResourceKindOptionsV7,
  kubernetesResourceKindOptionsV8,
  KubernetesResourceModel,
  kubernetesVerbOptions,
  newKubernetesResourceModel,
  ResourceAccess,
  ResourceAccessKind,
  ServerAccess,
  ServerAccessInputFields,
  supportsKubernetesCustomResources,
  WindowsDesktopAccess,
  WindowsDesktopAccessInputFields,
} from './standardmodel';
import { ActionType } from './useStandardModel';
import {
  AppAccessValidationResult,
  DatabaseAccessValidationResult,
  GitHubOrganizationAccessValidationResult,
  KubernetesAccessValidationResult,
  KubernetesResourceValidationResult,
  ResourceAccessValidationResult,
  ServerAccessValidationResult,
  v7kubernetesClusterWideResourceKinds,
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
  customDescription,
}: SectionPropsWithDispatch<
  ResourceAccess[],
  ResourceAccessValidationResult[]
> & {
  /**
   * Custom description describing this section.
   */
  customDescription?: React.ReactNode;
}) {
  /** All resource access kinds except those that are already in the role. */
  const allowedResourceAccessKinds = allResourceAccessKinds.filter(k =>
    value.every(as => as.kind !== k)
  );

  const addResourceAccess = (kind: ResourceAccessKind) =>
    dispatch({ type: ActionType.AddResourceAccess, payload: { kind } });

  return (
    <Flex flexDirection="column" gap={3}>
      {customDescription ? (
        <>{customDescription}</>
      ) : (
        <SectionPadding>
          Rules that allow connecting to resources controlled by Teleport
        </SectionPadding>
      )}
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
              Add Teleport Resource Access
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
    component: React.ComponentType<SectionProps<unknown, unknown, unknown>>;
  }
> = {
  kube_cluster: {
    title: 'Kubernetes Access',
    component: KubernetesAccessSection,
  },
  node: {
    title: 'SSH Server Access',
    component: ServerAccessSection,
  },
  app: {
    title: 'Application Access',
    component: AppAccessSection,
  },
  db: {
    title: 'Database Access',
    component: DatabaseAccessSection,
  },
  windows_desktop: {
    title: 'Windows Desktop Access',
    component: WindowsDesktopAccessSection,
  },
  git_server: {
    title: 'GitHub Organization Access',
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
  const { component: Body, title } = resourceAccessSections[value.kind];

  function handleChange(val: T) {
    dispatch({ type: ActionType.SetResourceAccess, payload: val });
  }

  function handleRemove() {
    dispatch({
      type: ActionType.RemoveResourceAccess,
      payload: { kind: value.kind },
    });
  }

  return (
    <ValidationSuspender suspend={value.hideValidationErrors}>
      <SectionBox
        titleSegments={[title]}
        removable
        onRemove={handleRemove}
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
    </ValidationSuspender>
  );
});

export function ServerAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
  readOnly = false,
  visibleInputFields,
}: SectionProps<
  ServerAccess,
  ServerAccessValidationResult,
  ServerAccessInputFields
>) {
  // Flags to conditionally render input fields.
  let show: ServerAccessInputFields = visibleInputFields ?? {
    labels: true,
    logins: true,
  };

  return (
    <>
      {show.labels && (
        <LabelsInput
          atLeastOneRow
          legend="Labels"
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange?.({ ...value, labels })}
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.labels)}
        />
      )}
      {show.logins && (
        <FieldSelectCreatable
          isMulti
          label="Logins"
          placeholder={readOnly ? '' : 'Type a login and press Enter'}
          isDisabled={isProcessing}
          formatCreateLabel={label => `Login: ${label}`}
          components={{
            DropdownIndicator: null,
          }}
          openMenuOnClick={false}
          value={value.logins}
          onChange={logins => onChange?.({ ...value, logins })}
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.logins)}
          mt={3}
          mb={0}
          menuPosition="fixed"
          toolTipContent={
            <>
              System usernames that can be used to log in to servers, e.g.{' '}
              <MarkInverse>ec2-user</MarkInverse>,{' '}
              <MarkInverse>ubuntu</MarkInverse>,{' '}
              <MarkInverse>centos</MarkInverse>.
            </>
          }
        />
      )}
    </>
  );
}

export function KubernetesAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
  readOnly = false,
  visibleInputFields,
}: SectionProps<
  KubernetesAccess,
  KubernetesAccessValidationResult,
  KubernetesAccessInputFields
>) {
  // Flags to conditionally render input fields.
  let show: KubernetesAccessInputFields = visibleInputFields ?? {
    labels: true,
    groups: true,
    users: true,
    resources: true,
  };

  const resourcesValidationResult = useRule(
    readOnly
      ? defaultRule()
      : precomputed(validation.fields.resources)(value.resources)
  );
  return (
    <>
      {show.groups && (
        <FieldSelectCreatable
          isMulti
          label="Groups"
          placeholder={readOnly ? '' : 'Type a group name and press Enter'}
          isDisabled={isProcessing}
          formatCreateLabel={label => `Group: ${label}`}
          components={{
            DropdownIndicator: null,
          }}
          openMenuOnClick={false}
          value={value.groups}
          onChange={groups => onChange?.({ ...value, groups })}
          menuPosition="fixed"
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.groups)}
        />
      )}

      {show.users && (
        <FieldSelectCreatable
          isMulti
          label="Users"
          placeholder={readOnly ? '' : 'Type a user name and press Enter'}
          isDisabled={isProcessing}
          formatCreateLabel={label => `User: ${label}`}
          components={{
            DropdownIndicator: null,
          }}
          openMenuOnClick={false}
          value={value.users}
          onChange={users => onChange?.({ ...value, users })}
          menuPosition="fixed"
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.users)}
        />
      )}

      {show.labels && (
        <LabelsInput
          atLeastOneRow
          legend="Labels"
          disableBtns={isProcessing}
          labels={value.labels}
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.labels)}
          setLabels={labels => onChange?.({ ...value, labels })}
        />
      )}

      {show.resources && (
        <Flex flexDirection="column" gap={3} mt={3}>
          {value.resources.map((resource, index) => (
            <KubernetesResourceView
              readOnly={readOnly}
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

          {!readOnly && (
            <Box>
              <Button
                fill={resourcesValidationResult.valid ? 'filled' : 'border'}
                intent={resourcesValidationResult.valid ? 'neutral' : 'danger'}
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
                size="small"
                $inputAlignment
              >
                <Add disabled={isProcessing} size="small" />
                {value.resources.length > 0
                  ? 'Add Another Kubernetes Resource'
                  : 'Add a Kubernetes Resource'}
              </Button>
            </Box>
          )}
        </Flex>
      )}
    </>
  );
}

function KubernetesResourceKindView({
  value,
  validation,
  isProcessing,
  onChange,
  roleVersion,
  readOnly = false,
}: {
  value: KubernetesResourceModel;
  validation: KubernetesResourceValidationResult['kind'];
  isProcessing: boolean;
  onChange?(m: KubernetesResourceModel): void;
  roleVersion: RoleVersion;
  readOnly?: boolean;
}) {
  if (!supportsKubernetesCustomResources(roleVersion)) {
    return (
      <FieldSelect
        label="Kind"
        isDisabled={isProcessing}
        options={kubernetesResourceKindOptionsV7.filter(
          elem => roleVersion == 'v7' || elem.value == 'pod' // In v7, we have the fill list, in v6 and earlier, only pod.
        )}
        value={value.kind}
        readOnly={readOnly}
        rule={readOnly ? undefined : precomputed(validation)}
        onChange={k => onChange?.({ ...value, kind: k })}
      />
    );
  }
  return (
    <FieldSelectCreatable
      isSearchable
      label="Kind (plural)"
      toolTipContent={
        <>
          Resource plural name, e.g. pods, deployments, mycustomresources.
          Special value <MarkInverse>*</MarkInverse> means any kind.
        </>
      }
      isDisabled={isProcessing}
      formatCreateLabel={label => `Kind: ${label}`}
      openMenuOnClick
      value={value.kind}
      onChange={kind => onChange?.({ ...value, kind })}
      menuPosition="fixed"
      readOnly={readOnly}
      rule={readOnly ? undefined : precomputed(validation)}
      options={kubernetesResourceKindOptionsV8}
      components={{
        DropdownIndicator: null,
      }}
    />
  );
}

function KubernetesResourceView({
  value,
  validation,
  isProcessing,
  onChange,
  onRemove,
  readOnly = false,
}: {
  value: KubernetesResourceModel;
  validation: KubernetesResourceValidationResult;
  isProcessing: boolean;
  onChange(m: KubernetesResourceModel): void;
  onRemove(): void;
  readOnly?: boolean;
}) {
  const { kind, name, namespace, verbs, apiGroup } = value;
  const theme = useTheme();
  const supportsCrds = supportsKubernetesCustomResources(value.roleVersion);
  return (
    <Box
      border={1}
      borderColor={theme.colors.interactive.tonal.neutral[0]}
      borderRadius={3}
      padding={3}
    >
      <Flex>
        <Box flex="1">
          <H4 mb={3}>Kubernetes Resource</H4>
        </Box>
        {!readOnly && (
          <ButtonIcon
            aria-label="Remove Kubernetes resource"
            disabled={isProcessing}
            onClick={onRemove}
          >
            <Trash
              size="small"
              color={theme.colors.interactive.solid.danger.default}
            />
          </ButtonIcon>
        )}
      </Flex>
      <KubernetesResourceKindView
        value={value}
        validation={validation.kind}
        isProcessing={isProcessing}
        onChange={k => onChange?.({ ...value, ...k })}
        roleVersion={value.roleVersion}
        readOnly={readOnly}
      />
      {(supportsCrds || apiGroup) && (
        <FieldInput
          label="API Group"
          required={!readOnly}
          toolTipContent={
            <>
              Resource API Group. Special value <MarkInverse>*</MarkInverse>{' '}
              means any group.
            </>
          }
          disabled={isProcessing}
          value={apiGroup}
          readonly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.apiGroup)}
          onChange={e => onChange?.({ ...value, apiGroup: e.target.value })}
        />
      )}
      <FieldInput
        label="Name"
        required={!readOnly}
        toolTipContent={
          <>
            Name of the resource. Special value <MarkInverse>*</MarkInverse>{' '}
            means any name.
          </>
        }
        disabled={isProcessing}
        value={name}
        readonly={readOnly}
        rule={readOnly ? undefined : precomputed(validation.name)}
        onChange={e => onChange?.({ ...value, name: e.target.value })}
      />
      <FieldInput
        label="Namespace"
        required={
          !readOnly &&
          !v7kubernetesClusterWideResourceKinds.includes(kind.value)
        }
        toolTipContent={
          <>
            Namespace that contains the resource. Special value{' '}
            <MarkInverse>*</MarkInverse> means any namespace.
          </>
        }
        disabled={isProcessing}
        value={namespace}
        readonly={readOnly}
        rule={readOnly ? undefined : precomputed(validation.namespace)}
        onChange={e => onChange?.({ ...value, namespace: e.target.value })}
      />
      <FieldSelect
        isMulti
        label="Verbs"
        isDisabled={isProcessing}
        options={kubernetesVerbOptions}
        value={verbs}
        readOnly={readOnly}
        rule={readOnly ? undefined : precomputed(validation.verbs)}
        onChange={v => onChange?.({ ...value, verbs: v })}
        mb={0}
        menuPosition="fixed"
      />
    </Box>
  );
}

export function AppAccessSection({
  value,
  validation,
  isProcessing,
  onChange,
  readOnly = false,
  visibleInputFields,
}: SectionProps<AppAccess, AppAccessValidationResult, AppAccessInputFields>) {
  // Flags to conditionally render input fields.
  let show: AppAccessInputFields = visibleInputFields ?? {
    labels: true,
    awsRoleARNs: true,
    azureIdentities: true,
    gcpServiceAccounts: true,
    mcpTools: true,
  };

  return (
    <Flex flexDirection="column" gap={3}>
      {show.labels && (
        <LabelsInput
          atLeastOneRow
          legend="Labels"
          disableBtns={isProcessing}
          labels={value.labels}
          setLabels={labels => onChange?.({ ...value, labels })}
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.labels)}
        />
      )}
      {show.awsRoleARNs && (
        <FieldMultiInput
          label="AWS Role ARNs"
          disabled={isProcessing}
          value={value.awsRoleARNs}
          onChange={arns => onChange?.({ ...value, awsRoleARNs: arns })}
          readOnly={readOnly}
          rule={
            readOnly ? undefined : precomputed(validation.fields.awsRoleARNs)
          }
          tooltipContent={
            <>
              List of AWS roles allowed to assume when accessing AWS console.
              Example format:{' '}
              <MarkInverse>
                arn:aws:iam::&lt;AWS_ACCOUNT&gt;:role/&lt;IAM_ROLE_NAME&gt;
              </MarkInverse>
            </>
          }
        />
      )}
      {show.azureIdentities && (
        <FieldMultiInput
          label="Azure Identities"
          disabled={isProcessing}
          value={value.azureIdentities}
          onChange={ids => onChange?.({ ...value, azureIdentities: ids })}
          readOnly={readOnly}
          rule={
            readOnly
              ? undefined
              : precomputed(validation.fields.azureIdentities)
          }
          tooltipContent={
            <>
              List of Azure managed identities allowed to assume when accessing
              Azure CLIs and APIs. Example format:
              <MarkInverse>
                /subscriptions/00000000-0000-0000-0000-000000000000{''}
                /resourceGroups/RESOURCE_GROUP_NAME /providers{''}
                /Microsoft.ManagedIdentity /userAssignedIdentities/IDENTITY_NAME
              </MarkInverse>
            </>
          }
        />
      )}
      {show.gcpServiceAccounts && (
        <FieldMultiInput
          label="GCP Service Accounts"
          disabled={isProcessing}
          value={value.gcpServiceAccounts}
          onChange={accts =>
            onChange?.({ ...value, gcpServiceAccounts: accts })
          }
          readOnly={readOnly}
          rule={
            readOnly
              ? undefined
              : precomputed(validation.fields.gcpServiceAccounts)
          }
          tooltipContent={
            <>
              List of Google Cloud Platform service accounts allowed to assume
              when accessing Google Cloud APIs.
            </>
          }
        />
      )}
      {show.mcpTools && (
        <FieldMultiInput
          label="MCP Tools"
          disabled={isProcessing}
          value={value.mcpTools}
          onChange={mcpTools => onChange?.({ ...value, mcpTools: mcpTools })}
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.mcpTools)}
          tooltipContent={
            <>
              List of MCP (Modern Content Protocol) tools allowed to access.
              Each entry can be a literal string (e.g.{' '}
              <MarkInverse>search-files</MarkInverse>
              ), a glob pattern (e.g. <MarkInverse>slack_*</MarkInverse>), or a
              regular expression that must start with &apos;^&apos; and end with
              &apos;$&apos; (e.g. <MarkInverse>^(get|list).*$</MarkInverse>).
              Special value <MarkInverse>*</MarkInverse> allows all tools.
            </>
          }
        />
      )}
    </Flex>
  );
}

export function DatabaseAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
  readOnly = false,
  visibleInputFields,
}: SectionProps<
  DatabaseAccess,
  DatabaseAccessValidationResult,
  DatabaseAccessInputFields
>) {
  // Flags to conditionally render input fields.
  let show: DatabaseAccessInputFields = visibleInputFields ?? {
    labels: true,
    names: true,
    users: true,
    roles: true,
    dbServiceLabels: true,
  };

  return (
    <>
      {show.labels && (
        <Box mb={3}>
          <LabelsInput
            atLeastOneRow
            legend="Labels"
            disableBtns={isProcessing}
            labels={value.labels}
            setLabels={labels => onChange?.({ ...value, labels })}
            readOnly={readOnly}
            rule={readOnly ? undefined : precomputed(validation.fields.labels)}
          />
        </Box>
      )}
      {show.names && (
        <FieldSelectCreatable
          isMulti
          label="Database Names"
          placeholder={readOnly ? '' : 'Type a database name and press Enter'}
          toolTipContent={
            <>
              Database names allowed to connect to. Special value{' '}
              <MarkInverse>*</MarkInverse> means any name.
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
          menuPosition="fixed"
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.names)}
        />
      )}
      {show.users && (
        <FieldSelectCreatable
          isMulti
          label="Database Users"
          placeholder={readOnly ? '' : 'Type a user name and press Enter'}
          toolTipContent={
            <>
              Database account names allowed to connect as. Special value{' '}
              <MarkInverse>*</MarkInverse> means any user.
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
          menuPosition="fixed"
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.users)}
        />
      )}
      {show.roles && (
        <FieldSelectCreatable
          isMulti
          label="Database Roles"
          placeholder={readOnly ? '' : 'Type a role name and press Enter'}
          toolTipContent="If automatic user provisioning is available, this is the list of database roles that will be assigned to the database user after it's created."
          isDisabled={isProcessing}
          formatCreateLabel={label => `Database Role: ${label}`}
          components={{
            DropdownIndicator: null,
          }}
          openMenuOnClick={false}
          value={value.roles}
          onChange={roles => onChange?.({ ...value, roles })}
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.roles)}
          menuPosition="fixed"
        />
      )}
      {show.dbServiceLabels && (
        <LabelsInput
          atLeastOneRow
          legend="Database Service Labels"
          tooltipContent="The database service labels control which Database Services (Teleport Agents) are visible to the user, which is required when adding Databases in the Enroll New Resource wizard. Access to Databases themselves is controlled by the Database Labels field."
          disableBtns={isProcessing}
          labels={value.dbServiceLabels}
          setLabels={dbServiceLabels =>
            onChange?.({ ...value, dbServiceLabels })
          }
          readOnly={readOnly}
          rule={
            readOnly
              ? undefined
              : precomputed(validation.fields.dbServiceLabels)
          }
        />
      )}
    </>
  );
}

export function WindowsDesktopAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
  readOnly = false,
  visibleInputFields,
}: SectionProps<
  WindowsDesktopAccess,
  WindowsDesktopAccessValidationResult,
  WindowsDesktopAccessInputFields
>) {
  // Flags to conditionally render input fields.
  const show: WindowsDesktopAccessInputFields = visibleInputFields ?? {
    labels: true,
    logins: true,
  };

  return (
    <>
      {show.labels && (
        <Box mb={3}>
          <LabelsInput
            atLeastOneRow
            legend="Labels"
            disableBtns={isProcessing}
            labels={value.labels}
            setLabels={labels => onChange?.({ ...value, labels })}
            readOnly={readOnly}
            rule={readOnly ? undefined : precomputed(validation.fields.labels)}
          />
        </Box>
      )}
      {show.logins && (
        <FieldSelectCreatable
          isMulti
          label="Logins"
          placeholder={readOnly ? '' : 'Type a login and press Enter'}
          toolTipContent="List of Windows logins allowed to use for desktop sessions."
          isDisabled={isProcessing}
          formatCreateLabel={label => `Login: ${label}`}
          components={{
            DropdownIndicator: null,
          }}
          openMenuOnClick={false}
          value={value.logins}
          onChange={logins => onChange?.({ ...value, logins })}
          menuPosition="fixed"
          readOnly={readOnly}
          rule={readOnly ? undefined : precomputed(validation.fields.logins)}
        />
      )}
    </>
  );
}

export function GitHubOrganizationAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
  readOnly = false,
  visibleInputFields,
}: SectionProps<
  GitHubOrganizationAccess,
  GitHubOrganizationAccessValidationResult,
  GitHubOrganizationAccessInputFields
>) {
  // Flags to conditionally render input fields.
  const show: GitHubOrganizationAccessInputFields = visibleInputFields ?? {
    organizations: true,
  };

  return (
    <>
      {show.organizations && (
        <FieldSelectCreatable
          isMulti
          label="Organization Names"
          toolTipContent="A list of GitHub organization names allowed access to."
          placeholder={
            readOnly ? '' : 'Type an organization name and press Enter'
          }
          isDisabled={isProcessing}
          formatCreateLabel={label => `Organization: ${label}`}
          components={{
            DropdownIndicator: null,
          }}
          openMenuOnClick={false}
          value={value.organizations}
          onChange={organizations => onChange?.({ ...value, organizations })}
          menuPosition="fixed"
          readOnly={readOnly}
          rule={
            readOnly ? undefined : precomputed(validation.fields.organizations)
          }
          mb={0}
        />
      )}
    </>
  );
}

// TODO(bl-nero): This should ideally use tonal neutral 1 from the opposite
// theme as background.
const MarkInverse = styled(Mark)`
  background: ${p => p.theme.colors.text.primaryInverse};
  color: ${p => p.theme.colors.text.main};
`;
