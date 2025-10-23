import { useEffect, useState } from 'react';

import { Alert, Box, Flex, H2, Indicator, Link, Text, Toggle } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  CreateFilters,
  FilterOption,
} from 'e-teleport/Integrations/shared/CreateFilters';
import cfg from 'teleport/config';
import { StyledBox } from 'teleport/Discover/Shared';
import { useTeleport } from 'teleport/index';
import userService, { User } from 'teleport/services/user';

import { Filters, FormDataField } from './types';

type UserOption = Option<User>;

export function FormMixin({ attempt }) {
  const ctx = useTeleport();

  const [authConnectorName, setAuthConnectorName] = useState('entra-id');

  const policyEnabled = cfg.isPolicyEnabled;
  const [accessGraphEnabled, setAccessGraphEnabled] = useState(policyEnabled);

  const [selectedOwners, setSelectedOwners] = useState<UserOption[]>([]);

  const userAccess = ctx.storeUser.getUserAccess();
  const canReadListUsers = userAccess.list && userAccess.read;

  const [userOptions, setUserOptions] = useState<UserOption[]>([]);

  const { attempt: fetchUserAttempt, run: fetchUsersRun } = useAttempt(
    canReadListUsers ? 'processing' : ''
  );

  useEffect(() => {
    if (canReadListUsers) {
      fetchUsers();
    }
  }, []);

  function fetchUsers() {
    fetchUsersRun(() =>
      userService.fetchUsers().then(fetchedUsers => {
        setUserOptions(fetchedUsers.map(u => ({ value: u, label: u.name })));
      })
    );
  }

  // An empty filter signifies "import all" behavior.
  // Filter toggle and input fields should be in sync because
  // the toggle is a fake state maintained in the UI and there
  // isn't any filter enable/disable option configured in the
  // plugin spec.
  const [hideFilters, setHideFilters] = useState(true);
  const [filters, setFilters] = useState<Filters>(emptyFilter);

  function toFilterOption(filters: string[]): FilterOption[] {
    // TODO(sshah): report invalid filters.
    return filters.map(f => ({ label: f, value: f, invalid: false }));
  }

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

      {fetchUserAttempt.status === 'failed' && (
        <Alert>{fetchUserAttempt.statusText}</Alert>
      )}
      {fetchUserAttempt.status !== 'processing' ? (
        <>
          <StyledBox mt={4}>
            <H2 mb={1}>Group Import Settings</H2>
            <Text mb={4}>
              Groups imported from the Microsoft Entra ID directory will be
              created as Access Lists and their respective group members will be
              created as an Access List member.
            </Text>
            <Text bold mb={1}>
              Group Filters
            </Text>
            <Text mb={2}>
              All groups are imported by default. Configure group filters to
              include or exclude groups matching a group ID or group display
              name. More details can be found in the &nbsp;
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
              <FieldSelectCreatable
                width="540px"
                autoFocus={true}
                placeholder="Type a username and press enter"
                isMulti
                isClearable
                isSearchable
                options={userOptions}
                onChange={(opts: UserOption[]) => setSelectedOwners(opts || [])}
                value={selectedOwners || []}
                noOptionsMessage={() => 'Type a username and press enter'}
                label="Add Default List Owner(s)"
                rule={requiredField('At least 1 default owner is required')}
                isDisabled={attempt.status === 'processing'}
              />
            </Box>
          </StyledBox>

          <StyledBox mt={4}>
            <H2 mb={1}>Access Graph Integration</H2>
            <Text mb={3}>
              Analyze your Entra ID directory and SSO applications using
              Teleport Access Graph.
            </Text>
            <Flex>
              <HoverTooltip
                placement="top"
                tipContent={
                  policyEnabled ? null : (
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
                  disabled={!policyEnabled || attempt.status === 'processing'}
                >
                  <Text ml={2}>Enable Access Graph integration</Text>
                </Toggle>
              </HoverTooltip>
            </Flex>
          </StyledBox>
        </>
      ) : (
        <Box m={4} textAlign="center">
          <Indicator delay="none" />
        </Box>
      )}
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

type filter = {
  name: keyof Filters;
  label: string;
  placeholder: string;
};

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

export const emptyFilter: Filters = {
  id: [],
  nameRegex: [],
  excludeId: [],
  excludeNameRegex: [],
};
