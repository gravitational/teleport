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

import { useCallback, useEffect, useState } from 'react';

import { Box, ButtonPrimary, ButtonSecondary, Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import { useTeleport } from 'teleport';
import { DbConnectData } from 'teleport/lib/term/tty';
import { Database } from 'teleport/services/databases';

export function ConnectDialog(props: {
  clusterId: string;
  serviceName: string;
  onClose(): void;
  onConnect(data: DbConnectData): void;
}) {
  // Fetch database information to pre-fill the connection parameters.
  const ctx = useTeleport();
  const [attempt, getDatabase] = useAsync(
    useCallback(async () => {
      const response = await ctx.resourceService.fetchUnifiedResources(
        props.clusterId,
        {
          query: `name == "${props.serviceName}"`,
          kinds: ['db'],
          sort: { fieldName: 'name', dir: 'ASC' },
          limit: 1,
        }
      );

      // TODO(gabrielcorado): Handle scenarios where there is conflict on the name.
      if (response.agents.length !== 1 || response.agents[0].kind !== 'db') {
        throw new Error('Unable to retrieve database information.');
      }

      return response.agents[0];
    }, [props.clusterId, ctx.resourceService, props.serviceName])
  );

  useEffect(() => {
    void getDatabase();
  }, [getDatabase]);

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={props.onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>Connect To Database</DialogTitle>
      </DialogHeader>

      {attempt.status === 'error' && <Danger children={attempt.statusText} />}
      {(attempt.status === '' || attempt.status === 'processing') && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <ConnectForm
          db={attempt.data}
          onConnect={props.onConnect}
          onClose={props.onClose}
        />
      )}
    </Dialog>
  );
}

function ConnectForm(props: {
  db: Database;
  onConnect(data: DbConnectData): void;
  onClose(): void;
}) {
  const dbUserOpts = props.db.users
    ?.map(user => ({
      value: user,
      label: user,
    }))
    .filter(removeWildcardOption);
  const dbNamesOpts = props.db.names
    ?.map(name => ({
      value: name,
      label: name,
    }))
    .filter(removeWildcardOption);
  const dbRolesOpts = props.db.roles
    ?.map(role => ({
      value: role,
      label: role,
    }))
    .filter(removeWildcardOption);

  const [selectedName, setSelectedName] = useState<Option>(dbNamesOpts?.[0]);
  const [selectedUser, setSelectedUser] = useState<Option>(dbUserOpts?.[0]);
  const [selectedRoles, setSelectedRoles] =
    useState<readonly Option[]>(dbRolesOpts);

  const dbConnect = () => {
    props.onConnect({
      serviceName: props.db.name,
      protocol: props.db.protocol,
      dbName: selectedName.value,
      dbUser: selectedUser.value,
      dbRoles: selectedRoles?.map(role => role.value),
    });
  };

  return (
    <Validation>
      {({ validator }) => (
        <form>
          <DialogContent flex="0 0 auto">
            <FieldSelectCreatable
              label="Database name"
              menuPosition="fixed"
              onChange={option => setSelectedName(option as Option)}
              value={selectedName}
              options={dbNamesOpts}
              formatCreateLabel={userInput =>
                `Use "${userInput}" database name`
              }
              rule={requiredField('Database name is required')}
            />
            <FieldSelectCreatable
              label="Database user"
              menuPosition="fixed"
              onChange={option => setSelectedUser(option as Option)}
              value={selectedUser}
              options={dbUserOpts}
              formatCreateLabel={userInput =>
                `Use "${userInput}" database user`
              }
              rule={requiredField('Database user is required')}
            />
            {dbRolesOpts?.length > 0 && (
              <FieldSelect
                label="Database roles"
                menuPosition="fixed"
                isMulti={true}
                onChange={setSelectedRoles}
                value={selectedRoles}
                options={dbRolesOpts}
                rule={requiredField('At least one database role is required')}
              />
            )}
          </DialogContent>
          <DialogFooter>
            <Flex alignItems="center" justifyContent="space-between">
              <ButtonSecondary
                type="button"
                width="45%"
                size="large"
                onClick={props.onClose}
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

function removeWildcardOption({ value }: Option): boolean {
  return value !== '*';
}

const dialogCss = () => `
  min-height: 200px;
  max-width: 600px;
  width: 100%;
`;
