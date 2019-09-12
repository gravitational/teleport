import React from 'react';
import { storiesOf } from '@storybook/react'
import AppTile from './AppTile';

storiesOf('GravityHub/HubCatalog', module)
  .add('AppTile', () => {
    return (
      <AppTile apps={apps}/>
    )}
  );

const apps = [
  {
    "id": "gravitational.io/alpine/0.1.0",
    "name": "alpine",
    "version": "0.1.0",
    "repository": "gravitational.io",
    "installUrl": "/web/installer/new/gravitational.io/alpine/0.1.0",
    "kind": "Application",
    "standaloneInstallerUrl": "/portalapi/v1/apps/gravitational.io/alpine/0.1.0/installer",
    "size": "7.22 MB",
    "created": "2019-04-23T16:58:57.451Z",
    "createdText": "23/04/2019 12:58:57",
  },
  {
    "id": "gravitational.io/alpine/0.2.0",
    "name": "alpine",
    "version": "0.2.0",
    "repository": "gravitational.io",
    "installUrl": "/web/installer/new/gravitational.io/alpine/0.1.0",
    "kind": "Application",
    "standaloneInstallerUrl": "/portalapi/v1/apps/gravitational.io/alpine/0.1.0/installer",
    "size": "7.22 MB",
    "created": "2018-04-23T16:58:57.451Z",
    "createdText": "23/04/2018 12:58:57",
  }
]