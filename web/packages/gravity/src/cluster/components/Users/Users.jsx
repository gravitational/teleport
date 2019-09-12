/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { useFluxStore } from 'gravity/components/nuclear';
import { SystemRoleEnum } from 'gravity/services/enums';
import { getters } from 'gravity/cluster/flux/users';
import * as storeActions from 'gravity/cluster/flux/users/actions';
import { useState, withState } from 'shared/hooks';
import { Box, ButtonPrimary } from 'design';
import UserList from './UserList';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';
import UserDeleteDialog from './UserDeleteDialog';
import UserInviteDialog from './UserInviteDialog';
import UserResetDialog from './UserResetDialog';
import UserEditDialog from './UserEditDialog';

function getUser(userStore, userId){
  return userStore.get('users').find( u => u.userId === userId);
}

export function Users(props) {
  const { usersStore, roles } = props;
  const [ userToDelete, setUserToDelete ] = useState(null);
  const [ showInviteDialog, setShowInviteDialog ] = useState(false);
  const [ userToReset, setUserToReset ] = useState(null);
  const [ userToEdit, setUserToEdit ] = useState(null);

  const onDelete = userId => {
     const selectedUser = getUser(usersStore, userId);
     setUserToDelete(selectedUser);
  }

  const onEdit = userId => {
    const selectedUser = getUser(usersStore, userId);
    setUserToEdit(selectedUser);
  }

  const onReset = userId => {
    const selectedUser = getUser(usersStore, userId);
    setUserToReset(selectedUser);
  }

  const OnInvite = () => {
    setShowInviteDialog(true);
  }

  const roleLabels = !roles ? [SystemRoleEnum.TELE_ADMIN] : roles.getRoleNames();

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          Users
        </FeatureHeaderTitle>
        <Box ml="auto" width="200px" alignSelf="center">
          <ButtonPrimary block  onClick={OnInvite}> INVITE USER</ButtonPrimary>
        </Box>
      </FeatureHeader>
      <UserList
        onReset={onReset}
        onEdit={onEdit}
        onDelete={onDelete}
        onAdd={OnInvite}
        roleLabels={roleLabels}
        users={usersStore.get('users').toJS()} />
        { userToDelete && (
          <UserDeleteDialog
            user={userToDelete}
            onClose={ () => setUserToDelete(null)}
          />
        )}
        { showInviteDialog && (
          <UserInviteDialog
            roles={roleLabels}
            onClose={ () => setShowInviteDialog(false)}
          />

        )}
        { userToReset && (
          <UserResetDialog
            onClose={ () => setUserToReset(null)}
            userId={userToReset.userId}
          />
        )}
        { userToEdit && (
          <UserEditDialog
            roles={roleLabels}
            user={userToEdit}
            onClose={ () => setUserToEdit(null)}
          />
        )}
    </FeatureBox>
  );
}

function mapState(){
  const usersStore = useFluxStore(getters.usersStore);
  return {
    usersStore,
    createInvite: storeActions.createInvite,
    deleteUser: storeActions.deleteUser,
    saveUser: storeActions.saveUser,
    resetUser: storeActions.resetUser,
  }
}

export default withState(mapState)(Users);