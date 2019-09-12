import React from 'react';
import { storiesOf } from '@storybook/react';
import DeleteConnectorDialog from './DeleteConnectorDialog';

const props = {
  name: 'sample-connector-role',
  onDelete: () => {
    return Promise.reject(new Error('server error'));
  },
  onClose: () => null,
};

storiesOf('Shared-E/AuthConnectors', module).add(
  'DeleteConnectorDialog',
  () => <DeleteConnectorDialog {...props} />
);
