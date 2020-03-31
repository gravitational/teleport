import React from 'react';
import DeleteDialog from './DeleteConnectorDialog';

export default {
  title: 'Shared-E',
};

export const DeleteConnectorDialog = () => <DeleteDialog {...props} />;

const props = {
  name: 'sample-connector-role',
  onDelete: () => {
    return Promise.reject(new Error('server error'));
  },
  onClose: () => null,
};

DeleteConnectorDialog.story = {
  name: 'DeleteConnectorDialog',
};
