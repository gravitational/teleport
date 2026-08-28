import { useState } from 'react';

import { Box, Flex, H2, Link, Text, Toggle } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectCreatableAsync } from 'shared/components/FieldSelect/FieldSelectCreatable';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import {
  useUserOptions,
  useUsersNoOptionsMessage,
} from 'e-teleport/AccessListManagement/Shared/hooks';
import {
  ReactSelectAccessListOption,
  UserDisplayNameMultiValueLabel,
} from 'e-teleport/AccessListManagement/Shared/Shared';
import { CreateFilters } from 'e-teleport/Integrations/shared/CreateFilters';
import cfg from 'teleport/config';
import { StyledBox } from 'teleport/Discover/Shared';
import { User } from 'teleport/services/user';

import { emptyFilter, filterCollection } from './SyncSettings/constants';
import { ConfigureSource, toFilterOption } from './SyncSettings/GroupsImport';
import { AccessListOwnersSource, Filters, FormDataField } from './types';

type UserOption = Option<User>;

export function FormMixin({ attempt }) {
  const [authConnectorName, setAuthConnectorName] = useState('entra-id');

  const accessGraphLicensed = cfg.entitlements.AccessGraph.enabled;
  const [accessGraphEnabled, setAccessGraphEnabled] =
    useState(accessGraphLicensed);

  const [selectedOwners, setSelectedOwners] = useState<UserOption[]>([]);

  const { loadOptions, canListUsers } = useUserOptions<UserOption>(
    (users: User[]) => users.map(u => ({ value: u, label: u.name }))
  );
  const noOptionsMessage = useUsersNoOptionsMessage(canListUsers);

  // An empty filter signifies "import all" behavior.
  // Filter toggle and input fields should be in sync because
  // the toggle is a fake state maintained in the UI and there
  // isn't any filter enable/disable option configured in the
  // plugin spec.
  const [hideFilters, setHideFilters] = useState(true);
  const [filters, setFilters] = useState<Filters>(emptyFilter);
  const [ownersSource, setOwnersSrouce] = useState<string>(
    AccessListOwnersSource.Plugin
  );

  return (
    <Box width="800px">
      <StyledBox>
        <H2 mb={1}>Configure SSO Connector</H2>
        <Text mb={3}>
          Users imported from the Microsoft Entra ID directory will use this SSO
          Connector to sign in to Teleport.
        </Text>
        <FieldInput
          width="540px"
          rule={requiredField('Auth connector name is required')}
          autoFocus={true}
          name={FormDataField.AuthConnectorName}
          value={authConnectorName}
          label="Give the SSO Connector a Name"
          placeholder="Auth connector name"
          onChange={e => setAuthConnectorName(e.target.value)}
          disabled={attempt.status === 'processing'}
          data-testid="auth-connector-name"
        />
      </StyledBox>

      <StyledBox mt={4}>
        <H2 mb={1}>Group Import Settings</H2>
        <Text mb={4}>
          Groups imported from the Microsoft Entra ID directory will be created
          as Access Lists and their respective group members will be created as
          an Access List member.
        </Text>
        <Text bold mb={1}>
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
          isToggled={hideFilters}
          onToggle={() => {
            if (!hideFilters) {
              // changing state from off to on should wipe out
              // filters because the "import all" behavior
              // requires zero configured filters.
              setFilters(emptyFilter);
            }
            setHideFilters(!hideFilters);
          }}
          size="small"
          disabled={attempt.status === 'processing'}
        >
          <Text ml={2}>Import All Groups</Text>
        </Toggle>

        {!hideFilters && (
          <Validation>
            {({ validator }) => (
              <Box mt={3}>
                {filterCollection.map(f => (
                  <CreateFilters
                    key={f.name}
                    filters={toFilterOption(filters[f.name])}
                    label={f.label}
                    validator={validator}
                    onFilterChange={e => {
                      setFilters({
                        ...filters,
                        [f.name]: e.map(input => input.value),
                      });
                    }}
                    placeholder={f.placeholder}
                    isDisabled={attempt.status === 'processing'}
                    autoFocus={f.name == 'id' ? true : false}
                  />
                ))}
              </Box>
            )}
          </Validation>
        )}

        <Box mt={5}>
          <Text bold mb={1}>
            Default Access Lists owner(s)
          </Text>
          <Text mb={3}>
            Access List owners are responsible for periodically reviewing
            membership to each Access List. At least one owner is required.
          </Text>
          <FieldSelectCreatableAsync
            width="540px"
            components={{
              Option: ReactSelectAccessListOption,
              MultiValueLabel: UserDisplayNameMultiValueLabel,
            }}
            autoFocus={true}
            placeholder="Type a username and press enter"
            isMulti
            isClearable
            isSearchable
            defaultOptions={true}
            loadOptions={loadOptions}
            onChange={(opts: UserOption[]) => setSelectedOwners(opts || [])}
            value={selectedOwners || []}
            noOptionsMessage={noOptionsMessage}
            label="Add Default List Owner(s)"
            rule={requiredField('At least 1 default owner is required')}
            isDisabled={attempt.status === 'processing'}
          />
        </Box>

        <Box mt={5}>
          <Text bold mb={1}>
            Access Lists owner(s) source
          </Text>
          <Text mb={3}>Configure source of the Access List owners.</Text>
          <ConfigureSource
            source={ownersSource}
            onSourceChange={setOwnersSrouce}
            disabled={attempt.status === 'processing'}
          />
        </Box>
      </StyledBox>

      <StyledBox mt={4}>
        <H2 mb={1}>Access Graph Integration</H2>
        <Text mb={3}>
          Analyze your Entra ID directory and SSO applications using Teleport
          Access Graph.
        </Text>
        <Flex>
          <HoverTooltip
            placement="top"
            tipContent={
              accessGraphLicensed ? null : (
                <Text>
                  You need Identity Security license to enable this feature.
                </Text>
              )
            }
          >
            <Toggle
              isToggled={accessGraphEnabled}
              onToggle={() => setAccessGraphEnabled(!accessGraphEnabled)}
              size="small"
              disabled={!accessGraphLicensed || attempt.status === 'processing'}
            >
              <Text ml={2}>Enable Access Graph integration</Text>
            </Toggle>
          </HoverTooltip>
        </Flex>
      </StyledBox>

      <input
        name={FormDataField.DefaultOwners}
        type="text"
        hidden
        readOnly={true}
        value={JSON.stringify(selectedOwners.map(o => o.label))}
      />
      <input
        name={FormDataField.AccessGraph}
        type="checkbox"
        hidden
        checked={accessGraphEnabled}
        readOnly={true}
      />
      <input
        name={FormDataField.GroupFilters}
        type="text"
        hidden
        readOnly={true}
        value={JSON.stringify(filters)}
      />
    </Box>
  );
}
