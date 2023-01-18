import React from 'react';

import { ClusterLogout } from './ClusterLogout';

export default {
  title: 'Teleterm/ModalsHost/ClusterLogout',
};

export const Story = () => {
  return (
    <ClusterLogout
      status=""
      statusText=""
      removeCluster={() => Promise.resolve([undefined, null])}
      onClose={() => {}}
      clusterUri="/clusters/foo"
      clusterTitle="foo"
    />
  );
};
