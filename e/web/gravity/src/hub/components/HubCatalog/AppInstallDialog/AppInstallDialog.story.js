import React from 'react'
import { storiesOf } from '@storybook/react'
import AppInstallDialog, { AppKindEnum }   from './AppInstallDialog';

storiesOf('GravityHub/AppInstallDialog', module)
  .add('ApplicationImage', () => {
    const props = {
      ...defaultProps,
      app
    }

    return (
      <AppInstallDialog {...props} />
    )
  })
  .add('ClusterImage', () => {
    const props = {
      ...defaultProps,
      app: {
        ...app,
        kind: AppKindEnum.CLUSTER
      }
    }

    return (
      <AppInstallDialog {...props} />
    )
  });

const defaultProps = {
  onClose: () => null
}

const app = {
  "id": "gravitational.io/alpine/0.1.0",
  "name": "alpine",
  "version": "0.1.0",
  "repository": "gravitational.io",
  "installUrl": "/web/installer/new/gravitational.io/alpine/0.1.0",
  "kind": AppKindEnum.APP,
  "standaloneInstallerUrl": "/portalapi/v1/apps/gravitational.io/alpine/0.1.0/installer",
  "size": "7.22 MB",
  "created": "2019-04-23T16:58:57.451Z",
  "createdText": "23/04/2019 12:58:57",
}