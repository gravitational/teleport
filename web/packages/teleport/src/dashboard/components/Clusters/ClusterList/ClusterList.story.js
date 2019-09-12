import React from 'react';
import { storiesOf } from '@storybook/react';
import ClusterList from './ClusterList';
import * as fixtures from './../fixtures';

storiesOf('TeleportDashboard/Clusters', module)
  .add('GridView', () => {
    return (
      <ClusterList mode="grid" clusters={fixtures.clusters} pageSizeGrid={5} />
    );
  })
  .add('TableView', () => {
    return (
      <ClusterList
        mode="table"
        clusters={fixtures.clusters}
        pageSizeTable={5}
      />
    );
  });
