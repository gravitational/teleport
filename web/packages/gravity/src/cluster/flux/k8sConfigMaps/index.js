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

import { toImmutable } from 'nuclear-js';
import reactor from 'gravity/reactor';
import store from './store';

const STORE_NAME = 'cluster_k8s_configmaps';

reactor.registerStores({ [STORE_NAME] : store });

const configMaps = [[STORE_NAME, 'configs'], configs => {
  let filteredConfigs = configs.map(itemMap => {
      let metadata =  itemMap.get('metadata') || toImmutable({});
      let {name, uid, namespace, creationTimestamp} = metadata.toJS();
      let data = getDataItems(itemMap.get('data'));
      return {
        name,
        id: uid,
        created: creationTimestamp,
        namespace,
        data
      }
    }).toJS();

  return filteredConfigs;
 }
];

function getDataItems(dataMap){
  let data = [];
  if(dataMap){
    dataMap.toKeyedSeq().forEach((item, key)=>{
      data.push({
        name: key,
        content: item
      })
    });
  }

  return data;
}

export const getters = {
  configMaps
}