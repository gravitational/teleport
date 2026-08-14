import { Box, Flex, Link, Text, Toggle } from 'design';
import { RadioGroup } from 'design/RadioGroup';
import { FieldSelectCreatableAsync } from 'shared/components/FieldSelect/FieldSelectCreatable';
import { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import {
  useUserOptions,
  useUsersNoOptionsMessage,
} from 'e-teleport/AccessListManagement/Shared/hooks';
import {
  CreateFilters,
  FilterOption,
} from 'e-teleport/Integrations/shared/CreateFilters';
import { type User } from 'teleport/services/user';

import {
  AccessListOwnersSource,
  Filters,
  toFrienldyAccessListOwnersSource,
} from '../types';
import { filterCollection } from './constants';
import type { UserOption } from './types';

export function AccessListOwners({
  selectedOptions,
  onOptionChange,
  accessListOwnersSource,
  onOwnersSourceChange,
  disabled,
}: {
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
  selectedOptions,
  onOptionChange,
  disabled,
}: {
  selectedOptions: UserOption[];
  onOptionChange: (UserOption) => void;
  disabled: boolean;
}) {
  const { loadOptions, canListUsers } = useUserOptions<UserOption>(
    (user: User[]) => {
      return user.map(user => ({
        label: user.name,
        value: user.name,
      }));
    }
  );
  const noOptionsMessage = useUsersNoOptionsMessage(canListUsers);
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
        placeholder="Type a username and press enter"
        isMulti
        isClearable
        isSearchable
        defaultOptions={true}
        loadOptions={loadOptions}
        value={selectedOptions}
        onChange={(opts: UserOption[]) => onOptionChange(opts)}
        noOptionsMessage={noOptionsMessage}
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

export function ConfigureFilters({
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
