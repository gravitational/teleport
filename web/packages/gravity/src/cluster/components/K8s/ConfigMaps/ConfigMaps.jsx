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
import { useFluxStore } from 'gravity/components/nuclear';
import { withState } from 'shared/hooks';
import { useK8sContext } from './../k8sContext';
import { getters } from 'gravity/cluster/flux/k8sConfigMaps';
import { saveConfigMap } from 'gravity/cluster/flux/k8sConfigMaps/actions';
import { Flex } from 'design';
import ConfigMapList from './ConfigMapList';
import ConfigMapEditor from './ConfigMapEditor';

export function ConfigMaps({ namespace, configMaps, onSaveMaps }) {
  const [ mapToEdit, setMapToEdit ] = React.useState(null);

  function onEdit(name){
    setMapToEdit(configMaps.find( m => m.name === name));
  }

  function onSave(changes) {
    return onSaveMaps(namespace, mapToEdit.name, changes)
  }

  const filtered = configMaps.filter(i => i.namespace === namespace);

  return (
    <Flex alignItems="start" flexWrap="wrap">
      <ConfigMapList namespace={namespace} items={filtered} onEdit={onEdit} />
      { mapToEdit && (
        <ConfigMapEditor
          configMap={mapToEdit}
          onSave={onSave}
          onClose={ () => setMapToEdit(null)}
        />
      )}
    </Flex>
  )
}

export default withState(() => {
  const configMaps = useFluxStore(getters.configMaps);
  const { namespace } = useK8sContext();
  return {
    namespace,
    configMaps,
    onSaveMaps: saveConfigMap,
  }
})(ConfigMaps);
