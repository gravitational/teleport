import React from 'react';
import { values } from 'lodash';
import { useFluxStore } from 'gravity/components/nuclear';
import CardEmpty from 'gravity/components/CardEmpty';
import { withState } from 'shared/hooks';
import { Flex } from 'design';
import { getters } from 'e-gravity/hub/flux/catalog';
import AppTileList from './AppTileList';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from '../components/Layout';

export function HubCatalog({ apps }){
  apps = apps || [];
  const isEmpty = apps.length === 0;
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          Catalog
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Flex flexWrap="wrap">
        { isEmpty && <CardEmpty title="There are no images"/> }
        { !isEmpty && <AppTileList apps={apps} /> }
      </Flex>
    </FeatureBox>
  )
}

function mapState(){
  const catalogStore = useFluxStore(getters.catalogStore);
  const apps = values(catalogStore.apps);
  return {
    apps,
  }
}

export default withState(mapState)(HubCatalog);