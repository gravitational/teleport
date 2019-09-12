import React from 'react'
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import DeleteRole from './DeleteRole';

const dialogProps = {
  siteId: 'fdf',
  role: { name: 'sample-role' },
  attempt: {
    isFailed: true,
    message: 'server error'
  },
  attemptActions: {

  },
  onDelete: () => {
    return $.Deferred().reject(new Error('server error'))
  },
  onClose: () => null
}

storiesOf('Gravity/Roles', module)
  .add('DeleteRole', () => (
    <DeleteRole {...dialogProps} />
  ));

