import { useState } from 'react';
import { useLocation, useNavigate } from 'react-router';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Link,
  Text,
  Toggle,
} from 'design';
import { RadioGroup } from 'design/RadioGroup';
import { FieldSelectCreatableAsync } from 'shared/components/FieldSelect/FieldSelectCreatable';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { useUserOptions } from 'e-teleport/AccessListManagement/Shared/hooks';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import {
  CreateFilters,
  FilterOption,
} from 'e-teleport/Integrations/shared/CreateFilters';
import cfg from 'teleport/config';
import { StyledBox } from 'teleport/Discover/Shared';
import type { Plugin, PluginEntraIdSpec } from 'teleport/services/integrations';
import { type User } from 'teleport/services/user';

import {
  AccessListOwnersSource,
  Filters,
  toFrienldyAccessListOwnersSource,
} from './types';

export function EditGroupsImport({
  plugin,
  onSave,
  disabled,
}: {
  plugin?: Plugin;
  onSave: (filters: Filters, owners: string[], ownersSource: string) => void;
  disabled: boolean;
}) {
  const navigate = useNavigate();
  const location = useLocation();

  const {
    filters,
    setFilters,
    selectedOwners,
    setSelectedOwners,
    loadOptions,
    accessListOwnersSource,
    setAccessListOwnersSource,
  } = useGroupImportsSettings(plugin);

  const [importAll, setImportAll] = useState(
    hasZeroFilters(plugin?.spec?.groupFilters)
  );

  function save(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    let filterValue = emptyFilter;
    if (!importAll) {
      // Toggle on (import all) state should wipe out
      // filters because the "import all" behavior
      // requires zero configured filters.
      filterValue = filters;
    }

    onSave(
      filterValue,
      selectedOwners.map(o => o.label),
      accessListOwnersSource
    );
  }

  function goBack() {
    if (!location.key || location.key === 'default') {
      navigate(cfg.getIntegrationStatusRoute('entra-id', plugin.name));
    } else {
      navigate(-1);
    }
  }

  return (
    <Box mt={3} width="800px">
      <Header header="Edit Group Import Settings" />
      <Text>
        Groups imported from the Microsoft Entra ID directory will be created as
        Access Lists <br /> and their respective group members will be created
        as an Access List member.
      </Text>
      <Text mb={4} mt={2}>
        Changes will be applied in the next import cycle.
      </Text>

      <Validation>
        {({ validator }) => (
          <>
            <StyledBox mt={4}>
              <ConfigureFilters
                enabled={importAll}
                setEnabled={setImportAll}
                filters={filters}
                onFilterChange={setFilters}
                validator={validator}
                disabled={disabled}
              />
            </StyledBox>

            <StyledBox mt={4}>
              <AccessListOwners
                loadOptions={loadOptions}
                selectedOptions={selectedOwners}
                onOptionChange={setSelectedOwners}
                accessListOwnersSource={accessListOwnersSource}
                onOwnersSourceChange={setAccessListOwnersSource}
                disabled={disabled}
              />
            </StyledBox>
            <Box mt={6} mb={6}>
              <ButtonPrimary onClick={() => save(validator)} mr={3}>
                Save
              </ButtonPrimary>
              <ButtonSecondary onClick={goBack}>Back</ButtonSecondary>
            </Box>
          </>
        )}
      </Validation>
    </Box>
  );
}

function AccessListOwners({
  loadOptions,
  selectedOptions,
  onOptionChange,
  accessListOwnersSource,
  onOwnersSourceChange,
  disabled,
}: {
  loadOptions: (input: string) => Promise<UserOption[]>;
  selectedOptions: UserOption[];
  onOptionChange: (UserOption) => void;
  accessListOwnersSource: string;
  onOwnersSourceChange: (string) => void;
  disabled: boolean;
}) {
  return (
    <>
      <Text bold typography="subtitle1" mb={1}>
        Access Lists owner(s)
      </Text>
      <Text>
        Access List owners are responsible for periodically reviewing membership
        to each Access List.
      </Text>
      <Flex gap={4} flexDirection="column" mt={4}>
        <DefaultOwners
          loadOptions={loadOptions}
          selectedOptions={selectedOptions}
          onOptionChange={onOptionChange}
          disabled={disabled}
        />
        <Box>
          <Text typography="subtitle2" mb={1}>
            Owners source
          </Text>
          <Text mb={3}>Configure source of the Access List owners.</Text>
          <ConfigureSource
            source={accessListOwnersSource}
            onSourceChange={onOwnersSourceChange}
            disabled={disabled}
          />
        </Box>
      </Flex>
    </>
  );
}

function DefaultOwners({
  loadOptions,
  selectedOptions,
  onOptionChange,
  disabled,
}: {
  loadOptions: (input: string) => Promise<UserOption[]>;
  selectedOptions: UserOption[];
  onOptionChange: (UserOption) => void;
  disabled: boolean;
}) {
  return (
    <Box maxWidth="600px">
      <Text typography="subtitle2" mb={1}>
        Default owners
      </Text>
      <Text mb={3}>
        Default owners will be used as Access List owners if Teleport is not
        configured to source group owners from Microsoft Entra ID or if
        Microsoft Entra ID group has zero supported owners.
      </Text>
      <FieldSelectCreatableAsync
        width="540px"
        required={true}
        autoFocus={true}
        placeholder="Type a username and press enter"
        isMulti
        isClearable
        isSearchable
        defaultOptions={true}
        loadOptions={loadOptions}
        value={selectedOptions}
        onChange={(opts: UserOption[]) => onOptionChange(opts)}
        noOptionsMessage={() => 'Type a username and press enter'}
        label="Add Default Owners"
        rule={requiredField('At least 1 default owner is required')}
        isDisabled={disabled}
      />
    </Box>
  );
}

export function ConfigureSource({
  source,
  onSourceChange,
  disabled,
}: {
  source: string;
  onSourceChange: (source: `${AccessListOwnersSource}`) => void;
  disabled: boolean;
}) {
  return (
    <Box maxWidth="600px">
      <RadioGroup
        size="small"
        name="accessListOwnersSource"
        onChange={(v: `${AccessListOwnersSource}`) => onSourceChange(v)}
        value={source}
        options={[
          {
            value: AccessListOwnersSource.Plugin,
            label: toFrienldyAccessListOwnersSource(
              AccessListOwnersSource.Plugin
            ),
            helperText: 'Use default owners as Access List owners.',
            disabled: disabled,
          },
          {
            value: AccessListOwnersSource.EntraId,
            label: toFrienldyAccessListOwnersSource(
              AccessListOwnersSource.EntraId
            ),
            helperText: `Use Microsoft Entra ID group owners as Access List owners. Only the
            group owner of User type is supported. If the Microsoft Entra ID group has zero owners, Teleport will
            fallback to using plugin source.`,
            disabled: disabled,
          },
          {
            value: AccessListOwnersSource.PluginAndEntraId,
            label: toFrienldyAccessListOwnersSource(
              AccessListOwnersSource.PluginAndEntraId
            ),
            helperText:
              'Use both plugin source and Microsoft Entra ID source to configure Access List owners.',
            disabled: disabled,
          },
        ]}
      />
    </Box>
  );
}

function ConfigureFilters({
  enabled,
  setEnabled,
  filters,
  onFilterChange,
  validator,
  disabled,
}: {
  enabled: boolean;
  setEnabled: (boolean) => void;
  filters: Filters;
  onFilterChange: (Filters) => void;
  validator: Validator;
  disabled: boolean;
}) {
  return (
    <Box>
      <Text bold typography="subtitle1" mb={1}>
        Group Filters
      </Text>
      <Text mb={2}>
        All groups are imported by default. Configure group filters to include
        or exclude groups matching a group ID or group display name. More
        details can be found in the &nbsp;
        <Link
          target="_blank"
          href="https://goteleport.com/docs/identity-governance/integrations/entra-id/advanced-options/#group-filters"
        >
          docs
        </Link>
        .
      </Text>
      <Toggle
        data-testid="testid"
        isToggled={enabled}
        onToggle={() => {
          setEnabled(!enabled);
        }}
        size="small"
        disabled={disabled}
      >
        <Text ml={2}>Import All Groups</Text>
      </Toggle>

      {!enabled && (
        <Box mt={3}>
          {filterCollection.map(f => (
            <CreateFilters
              key={f.name}
              filters={toFilterOption(filters[f.name])}
              label={f.label}
              validator={validator}
              onFilterChange={e => {
                onFilterChange({
                  ...filters,
                  [f.name]: e.map(input => input.value),
                });
              }}
              placeholder={f.placeholder}
              autoFocus={f.name == 'id' ? true : false}
              isDisabled={disabled}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}

type filter = {
  name: keyof Filters;
  label: string;
  placeholder: string;
};

/**
 * toFilterOption converts filter input array to [FilterOption].
 */
export function toFilterOption(filters: string[]): FilterOption[] {
  if (!filters) {
    return [];
  }
  // TODO(sshah): report invalid filters.
  return filters.map(f => ({ label: f, value: f, invalid: false }));
}

/**
 * filterCollection defines the supported filter modes for the Entra ID plugin.
 */
export const filterCollection: filter[] = [
  {
    name: 'id',
    label: 'Include Groups Matching the Specified Group IDs',
    placeholder: 'Type a group ID and press enter',
  },
  {
    name: 'nameRegex',
    label:
      'Include Groups Matching the Specified Group Name(s) - Regex and Glob Supported',
    placeholder:
      'Type a group name, regex or glob matching group name(s) and press enter',
  },
  {
    name: 'excludeId',
    label: 'Exclude Groups Matching the Specified Group IDs',
    placeholder: 'Type a group ID and press enter',
  },
  {
    name: 'excludeNameRegex',
    label:
      'Exclude Groups Matching the Specified Group Name(s) - Regex and Glob Supported',
    placeholder:
      'Type a group name, regex or glob matching group name(s) and press enter',
  },
];

export function hasZeroFilters(filters: Filters): boolean {
  if (!filters) {
    return true;
  }
  const hasFilters =
    filters.id?.length > 0 ||
    filters.nameRegex?.length > 0 ||
    filters.excludeId?.length > 0 ||
    filters.excludeNameRegex?.length > 0;

  return !hasFilters;
}

type UserOption = Option<string, string>;

function useGroupImportsSettings(plugin?: Plugin<PluginEntraIdSpec>) {
  function toUserOption(owners: string[]): UserOption[] {
    if (!owners) {
      return [];
    }
    return owners.map(u => ({ value: u, label: u }));
  }

  const [selectedOwners, setSelectedOwners] = useState<UserOption[]>(
    toUserOption(plugin?.spec.defaultOwners)
  );

  const [filters, setFilters] = useState<Filters>(
    plugin?.spec?.groupFilters ? plugin?.spec?.groupFilters : emptyFilter
  );

  const [accessListOwnersSource, setAccessListOwnersSource] = useState(
    plugin?.spec?.accessListOwnersSource ?? AccessListOwnersSource.Plugin
  );

  const { loadOptions } = useUserOptions<UserOption>((user: User[]) => {
    return user.map(user => ({
      label: user.name,
      value: user.name,
    }));
  });

  return {
    loadOptions,
    selectedOwners,
    setSelectedOwners,
    filters,
    setFilters,
    accessListOwnersSource,
    setAccessListOwnersSource,
  };
}

export const emptyFilter: Filters = {
  id: [],
  nameRegex: [],
  excludeId: [],
  excludeNameRegex: [],
};
