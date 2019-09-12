import React from 'react'
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import { DeleteConnectorDialog } from './DeleteConnectorDialog';

const dialogProps = {
  connector: { name: 'sample-connector-role' },
  onDelete: () => {
    return $.Deferred().reject(new Error('server error'))
  },
  onClose: () => null
}

storiesOf('Gravity/AuthConnectors', module)
  .add('DeleteConnectorDialog', () => (
    <DeleteConnectorDialog {...dialogProps} />
  ));

