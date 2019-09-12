import React from 'react';
import { storiesOf } from '@storybook/react';
import DeleteTrustedClusterDialog from './DeleteTrustedClusterDialog';

const props = {
  name: 'sample-trusted-cluster',
  onDelete: () => {
    return Promise.reject(new Error('server error'));
  },
  onClose: () => null,
};

storiesOf('Teleport/TrustedClusters', module).add(
  'DeleteTrustedClusterDialog',
  () => <DeleteTrustedClusterDialog {...props} />
);
