import React, { useEffect, useState } from 'react';

import { Text, Alert, Box, Indicator } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'teleport/config';
import { useTeleport } from 'teleport/index';
import userService, { User } from 'teleport/services/user';

import { FieldCheckbox } from 'shared/components/FieldCheckbox';

import { H2 } from 'design';

import { FormDataField } from './types';

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

  return (
    <Box width="800px">
      <FieldInput
        rule={requiredField('Auth connector name is required')}
        autoFocus={true}
        name={FormDataField.AuthConnectorName}
        value={authConnectorName}
        label="Give the SSO connector a name"
        placeholder="Auth connector name"
        onChange={e => setAuthConnectorName(e.target.value)}
        disabled={attempt.status === 'processing'}
        data-testid="auth-connector-name"
      />

      <H2 mb={1}>Set default owner(s) for imported Access Lists</H2>
      <Text>
        List Owners are responsible for periodically reviewing membership to
        each Access List. You must assign at least 1 default owner to your
        imported access lists.
      </Text>
      {fetchUserAttempt.status === 'failed' && (
        <Alert>{fetchUserAttempt.statusText}</Alert>
      )}
      {fetchUserAttempt.status !== 'processing' ? (
        <>
          <Box width="540px" mt={2}>
            <FieldSelectCreatable
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
          <Box my={4}>
            <FieldCheckbox
              label="Enable Access Graph integration"
              name={FormDataField.AccessGraph}
              checked={accessGraphEnabled}
              disabled={!policyEnabled || attempt.status === 'processing'}
              onChange={e => {
                setAccessGraphEnabled(e.target.checked);
              }}
            />
          </Box>
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
    </Box>
  );
}
