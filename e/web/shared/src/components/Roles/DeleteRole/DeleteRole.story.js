import React from 'react';
import { storiesOf } from '@storybook/react';
import DeleteRole from './DeleteRole';

const props = {
  name: 'sample-role',
  onDelete: () => {
    return Promise.reject(new Error('server error'));
  },
  onClose: () => null,
};

storiesOf('Shared-E/Roles', module).add('DeleteRoleDialog', () => (
  <DeleteRole {...props} />
));
