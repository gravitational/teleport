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

import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import { Add, Trash } from 'design/Icon';
import { Mark } from 'design/Mark';
import Text, { H4 } from 'design/Text';
import FieldInput from 'shared/components/FieldInput';
import { FieldMultiInput } from 'shared/components/FieldMultiInput/FieldMultiInput';
import FieldSelect, {
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { precomputed } from 'shared/components/Validation/rules';
import styled, { useTheme } from 'styled-components';
import { LabelsInput } from 'teleport/components/LabelsInput';

import { SectionProps, SectionBox } from './sections';
import {
  ResourceAccessKind,
  ResourceAccess,
  ServerAccess,
  KubernetesAccess,
  newKubernetesResourceModel,
  KubernetesResourceModel,
  kubernetesResourceKindOptions,
  kubernetesVerbOptions,
  AppAccess,
  DatabaseAccess,
  WindowsDesktopAccess,
} from './standardmodel';
import {
  ResourceAccessValidationResult,
  ServerAccessValidationResult,
  KubernetesAccessValidationResult,
  KubernetesResourceValidationResult,
  AppAccessValidationResult,
  DatabaseAccessValidationResult,
  WindowsDesktopAccessValidationResult,
} from './validation';

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
};

/**
 * A generic resource section. Details are rendered by components from the
 * `resourceAccessSections` map.
 */
export const ResourceAccessSection = <
  T extends ResourceAccess,
  V extends ResourceAccessValidationResult,
>({
  value,
  isProcessing,
  validation,
  onChange,
  onRemove,
}: SectionProps<T, V> & {
  onRemove?(): void;
}) => {
  const {
    component: Body,
    title,
    tooltip,
  } = resourceAccessSections[value.kind];
  return (
    <SectionBox
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
    </SectionBox>
  );
};

export function ServerAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<ServerAccess, ServerAccessValidationResult>) {
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
          <Trash
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

export function AppAccessSection({
  value,
  validation,
  isProcessing,
  onChange,
}: SectionProps<AppAccess, AppAccessValidationResult>) {
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

export function DatabaseAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<DatabaseAccess, DatabaseAccessValidationResult>) {
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

export function WindowsDesktopAccessSection({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<WindowsDesktopAccess, WindowsDesktopAccessValidationResult>) {
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

// TODO(bl-nero): This should ideally use tonal neutral 1 from the opposite
// theme as background.
const MarkInverse = styled(Mark)`
  background: ${p => p.theme.colors.text.primaryInverse};
  color: ${p => p.theme.colors.text.main};
`;
