import React from 'react';
import DeleteTrustedCluster from './DeleteTrustedClusterDialog';

export default {
  title: 'TeleportE/TrustedClusters',
};

export const DeleteTrustedClusterDialog = () => (
  <DeleteTrustedCluster {...props} />
);

DeleteTrustedClusterDialog.story = {
  name: 'DeleteDialog',
};

const props = {
  name: 'sample-trusted-cluster',
  onDelete: () => {
    return Promise.reject(new Error('server error'));
  },
  onClose: () => null,
};
