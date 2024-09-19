import React, { useState } from 'react';

import { Flex, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import Validation from 'shared/components/Validation';
import { Option } from 'shared/components/Select';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Database } from 'teleport/services/databases';

export default function({ db, onClose, onConnect }: Props) {
  // TODO: handler this case.
  if (db === undefined) {
    return
  }

  const dbUserOpts = db?.users.map(user => ({value: user, label: user}));
  const dbNamesOpts = db?.names.map(name => ({value: name, label: name}));
  const dbRolesOpts = db?.roles.map(role => ({value: role, label: role}));

  const [selectedUser, setSelectedUser] = useState<Option>(dbUserOpts[0]);
  const [selectedRoles, setSelectedRoles] = useState<readonly Option[]>(dbRolesOpts);
  const [selectedName, setSelectedName] = useState<Option>(dbNamesOpts[0]);

  const connect = () => {
    onConnect(selectedName.value, selectedUser.value, selectedRoles.map((role) => role.value))
  }

  return <Dialog
    dialogCss={dialogCss}
    disableEscapeKeyDown={false}
    onClose={onClose}
    open={true}
  >
    <Validation>
      {({ validator }) => ( <>
        <DialogHeader mb={4}>
          <DialogTitle>Connect To Database</DialogTitle>
        </DialogHeader>

        <DialogContent minHeight="240px" flex="0 0 auto">
          <FieldSelect
            label="Database user"
            menuPosition="fixed"
            onChange={option => setSelectedUser(option as Option)}
            value={selectedUser}
            options={dbUserOpts}
            isDisabled={dbUserOpts.length == 1}
          />
          {dbRolesOpts.length > 0 && <FieldSelect
            label="Database roles"
            menuPosition="fixed"
            isMulti={true}
            onChange={setSelectedRoles}
            value={selectedRoles}
            options={dbRolesOpts}
          /> }
          <FieldSelect
            label="Database name"
            menuPosition="fixed"
            onChange={option => setSelectedName(option as Option)}
            value={selectedName}
            options={dbNamesOpts}
            isDisabled={dbNamesOpts.length == 1}
          />
        </DialogContent>
        <DialogFooter>
          <Flex alignItems="center" justifyContent="space-between">
            <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
            <ButtonPrimary onClick={e => {
                e.preventDefault();
                validator.validate() && connect();
              }}>
                Connect
              </ButtonPrimary>
            </Flex>
        </DialogFooter>
      </>
    )}
    </Validation>
  </Dialog>
}

export type Props = {
  db?: Database;
  onClose: () => void;
  onConnect: (
    dbName: string,
    dbUser: string,
    dbRoles: string[],
  ) => void;
};

const dialogCss = () => `
  min-height: 200px;
  max-width: 600px;
  width: 100%;
`;
