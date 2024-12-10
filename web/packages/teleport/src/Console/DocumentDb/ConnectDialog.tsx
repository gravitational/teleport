/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useCallback, useEffect, useState } from 'react';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import {
    Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
} from 'design';

import Validation from 'shared/components/Validation';
import { Option } from 'shared/components/Select';
import { FieldSelect, FieldSelectCreatable } from 'shared/components/FieldSelect';
import { Database } from 'teleport/services/databases';
import { useTeleport } from 'teleport';
import { useUnifiedResourcesFetch } from 'shared/components/UnifiedResources';
import { Danger } from 'design/Alert';
import { requiredField } from 'shared/components/Validation/rules';

type Props = {
  clusterId: string;
  serviceName: string;
  onClose(): void;
  onConnect: onConnectCallback;
};

type onConnectCallback = (
  name: string,
  protocol: string,
  dbName: string,
  dbUser: string,
  dbRoles: string[],
) => void;

function DbConnectDialog({ clusterId, serviceName, onClose, onConnect }: Props) {
  // Fetch database information to pre-fill the connection parameters.
  const ctx = useTeleport();
  const {
    fetch: unifiedFetch,
    attempt,
    resources,
  } = useUnifiedResourcesFetch({
    fetchFunc: useCallback(
      async (_, signal) => {
        const response = await ctx.resourceService.fetchUnifiedResources(
          clusterId,
          {
            query: `name == "${serviceName}"`,
            sort: { fieldName: 'name', dir: 'ASC'},
            limit: 1,
          },
          signal
        );


        // TODO(gabrielcorado): Handle scenarios where there is conflict on the name.
        if (response.agents.length !== 1 || response.agents[0].kind !== 'db') {
          throw new Error('Unable to retrieve database information.');
        }

        return { agents: [response.agents[0] as Database] };
      },
      [clusterId, serviceName]
    )
  })
  useEffect(() => { unifiedFetch({clear: true})}, [])


  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>Connect To Database</DialogTitle>
      </DialogHeader>

      {attempt.status === 'failed' && <Danger children={attempt.statusText} />}
      {(attempt.status === '' || attempt.status === 'processing') && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && <DbConnectDialogForm db={resources[0]} onConnect={onConnect} onClose={onClose} />}
    </Dialog>
  );
}

type FormProps = {
  db: Database;
  onConnect: onConnectCallback;
  onClose(): void;
};

function DbConnectDialogForm({ db, onConnect, onClose }: FormProps) {
  const dbUserOpts = db.users?.map(user => ({value: user, label: user}));
  const dbNamesOpts = db.names?.map(name => ({value: name, label: name}));
  const dbRolesOpts = db.roles?.map(role => ({value: role, label: role}));

  const [selectedName, setSelectedName] = useState<Option>(dbNamesOpts?.[0]);
  const [selectedUser, setSelectedUser] = useState<Option>(dbUserOpts?.[0]);
  const [selectedRoles, setSelectedRoles] = useState<readonly Option[]>();

  const dbConnect = () => {
    onConnect(db.name, db.protocol, selectedName.value, selectedUser.value, selectedRoles?.map((role) => role.value));
  };

  return (
    <Validation>
      {({ validator }) => (
        <form>
          <DialogContent minHeight="240px" flex="0 0 auto">
            <FieldSelectCreatable
              label="Database user"
              menuPosition="fixed"
              onChange={option => setSelectedUser(option as Option)}
              value={selectedUser}
              options={dbUserOpts}
              isDisabled={dbUserOpts?.length == 1}
              formatCreateLabel={userInput => `Use "${userInput}" database user`}
              rule={requiredField('Database user is required')}
            />
            {dbRolesOpts?.length > 0 && <FieldSelect
              label="Database roles"
              menuPosition="fixed"
              isMulti={true}
              onChange={setSelectedRoles}
              value={selectedRoles}
              options={dbRolesOpts}
            /> }
            <FieldSelectCreatable
              label="Database name"
              menuPosition="fixed"
              onChange={option => setSelectedName(option as Option)}
              value={selectedName}
              options={dbNamesOpts}
              isDisabled={dbNamesOpts?.length == 1}
              formatCreateLabel={userInput => `Use "${userInput}" database name`}
              rule={requiredField('Database name is required')}
            />
          </DialogContent>
          <DialogFooter>
            <Flex alignItems="center" justifyContent="space-between">
              <ButtonSecondary
                type="button"
                width="45%"
                size="large"
                onClick={onClose}
              >
                Close
              </ButtonSecondary>
              <ButtonPrimary
                type="submit"
                width="45%"
                size="large"
                onClick={e => {
                  e.preventDefault();
                  validator.validate() && dbConnect();
                }}
              >
                Connect
              </ButtonPrimary>
            </Flex>
          </DialogFooter>
        </form>
      )}
    </Validation>
  );
}

const dialogCss = () => `
  min-height: 200px;
  max-width: 600px;
  width: 100%;
`;

export default DbConnectDialog;
