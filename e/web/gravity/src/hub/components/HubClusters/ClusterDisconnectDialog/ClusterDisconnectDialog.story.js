import React from 'react'
import { storiesOf } from '@storybook/react'
import { ClusterDisconnectDialog } from './ClusterDisconnectDialog';

storiesOf('GravityHub/ClusterDisconnectDialog', module)
  .add('ClusterDisconnectDialog', () => (
    <ClusterDisconnectDialog {...dialogProps} />
  ))
  .add('Error', () => (
    <ClusterDisconnectDialog { ...dialogProps } attempt={{isFailed: true, message: serverError}}
      />
  ));

const serverError = "this is a long error message which should be wrapped";

const dialogProps = {
  cluster: {
    siteId: 'cluster_name',
  },
  attempt: { },
  attemptActions: { },
  onClose: () => null
}
