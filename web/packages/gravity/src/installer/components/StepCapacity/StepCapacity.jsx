/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { map, findIndex } from 'lodash';
import { StepLayout } from '../Layout';
import FlavorSelector from './FlavorSelector';
import { useInstallerContext } from './../store';
import Flavor from './Flavor';

export default function StepCapacity() {
  const store = useInstallerContext();
  const flavorOptions = store.state.flavors.options;
  const agentServers = store.state.agentServers;

  // selected flavor
  const [ selectedFlavor, setSelectedFlavor ] = React.useState(() => {
    const index = findIndex(flavorOptions, f => f.isDefault === true);
    return index !== -1 ? index : 0;
  });

  // slider options
  const sliderOptions = React.useMemo( () => {
    return map(store.state.flavors.options, f => ({
      value: f.name,
      label: f.title
    }))
  });

  // profiles of selected flavor
  const profiles = React.useMemo( () => {
    if(flavorOptions[selectedFlavor]){
      const p = flavorOptions[selectedFlavor].profiles;
      // set new profile to configure from given flavor
      store.setProvisionProfiles(p)
      return p;
    }

    return [];
   }, [selectedFlavor]);


  function onChangeFlavor(index){
    setSelectedFlavor(index);
  }

  return (
    <StepLayout title={store.state.flavors.prompt || "Review Infrastructure Requirements" }>
      <FlavorSelector
        current={selectedFlavor}
        options={sliderOptions}
        onChange={onChangeFlavor}
      />
      <Flavor
        servers={agentServers}
        key={selectedFlavor}
        profiles={profiles}
        store={store}
      />
    </StepLayout>
  );
}