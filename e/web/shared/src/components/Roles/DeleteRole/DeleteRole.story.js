import React from 'react';
import DeleteRole from './DeleteRole';

export default {
  title: 'Shared-E',
};

export const DeleteRoleDialog = () => <DeleteRole {...props} />;
DeleteRoleDialog.story = {
  name: 'DeleteRoleDialog',
};

const props = {
  name: 'sample-role',
  onDelete: () => {
    return Promise.reject(new Error('server error'));
  },
  onClose: () => null,
};
