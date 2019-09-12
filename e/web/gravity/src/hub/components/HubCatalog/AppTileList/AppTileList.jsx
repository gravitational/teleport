import React from 'react';
import { sortBy, values, groupBy } from 'lodash';
import { Flex } from 'design';
import AppTile from './../AppTile';
import AppInstallDialog from './../AppInstallDialog';

export default function AppTileList({apps}){
  const [ appToInstall, setAppToInstall ] = React.useState(null);

  // sort by date before grouping
  const sorted = sortBy(apps, 'created').reverse();

  // group by name and repository
  const grouped = values(groupBy(sorted, i => i.name+i.repo))
    .map(v => ({
      id: v[0].id,
      name: v[0].name,
      created: v[0].created,
      apps: v
    })
  );

  // sort groups by date
  const sortedGroups = sortBy(grouped, 'created').reverse();

  function onAppInstall(app){
    setAppToInstall(app);
  }

  function onAppInstallClose(){
    setAppToInstall(null);
  }

  const $apps = sortedGroups.map(item => (
    <AppTile mr="5" mb="5" key={item.id}
      onInstall={onAppInstall}
      apps={item.apps}
    />
  ));

  return  (
    <>
      <Flex flexWrap="wrap">
        {$apps}
      </Flex>
      {appToInstall && <AppInstallDialog onClose={onAppInstallClose} app={appToInstall} />}
    </>
  )
}

