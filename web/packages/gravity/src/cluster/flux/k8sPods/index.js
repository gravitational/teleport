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

import cfg from 'gravity/config';
import { K8sPodPhaseEnum, K8sPodDisplayStatusEnum } from 'gravity/services/enums'
import reactor from 'gravity/reactor';
import store from './store';

const STORE_NAME = 'cluster_k8s_pods';

reactor.registerStores({[STORE_NAME] : store });

const podInfoList = [[STORE_NAME], podsMap => {
    const siteId = cfg.defaultSiteId;
    return podsMap.valueSeq()
      .filter(itemMap => itemMap.getIn(['status', 'phase']) !== K8sPodPhaseEnum.SUCCEEDED)
      .map(itemMap => {
        const name = itemMap.getIn(['metadata','name']);
        const namespace = itemMap.getIn(['metadata', 'namespace']);
        const podLogUrl = cfg.getSiteLogQueryRoute({ query: `pod:${name}` });
        const podMonitorUrl = cfg.getSiteK8sPodMonitorRoute(siteId, namespace, name);
        const { status, statusDisplay } = getStatus(itemMap);
        return {
          containerNames: getContainerNames(itemMap),
          containers: getContainers(itemMap),
          labelsText: getLabelsText(itemMap),
          name,
          namespace,
          phaseValue: itemMap.getIn(['status', 'phase']),
          podHostIp: itemMap.getIn(['status','hostIP']),
          podIp: itemMap.getIn(['status','podIP']),
          podLogUrl,
          resourceMap: itemMap,
          podMonitorUrl,
          status,
          statusDisplay,
        }
      })
      .toJS();
  }
];

export const getters = {
  podInfoList
}

// helpers
function createContainerStatus(containerMap){
  let phaseText = 'unknown';
  if(containerMap.getIn(['state', 'running'])){
    phaseText = 'running';
  }

  const name = containerMap.get('name');
  const logUrl = cfg.getSiteLogQueryRoute({ query: `container:${name}` });
  return {
    name,
    logUrl,
    phaseText
  }
}

function getLabelsText(pod){
  let labelMap = pod.getIn(['metadata', 'labels']);
  if(!labelMap){
    return [];
  }

  let results = [];
  let withAppAndName = [];

  labelMap.entrySeq().forEach(item => {
    let [labelName, lavelValue] = item;
    let text = labelName+':'+ lavelValue;
    if(labelName === 'app' || labelName === 'name' ){
      withAppAndName.push(text);
    }else{
      results.push(text);
    }
  })

 return withAppAndName.concat(results);

}

function getContainers(pod){
  const statusList = pod.getIn(['status', 'containerStatuses']);
  if(!statusList){
    return [];
  }

  return statusList
    .map(createContainerStatus)
    .toArray() || [];
}

function getContainerNames(podMap){
  const containerList = podMap.getIn(['spec', 'containers']);
  if(!containerList){
    return [];
  }

  return containerList
    .map(item=> item.get('name'))
    .toArray() || [];
}


function getStatus(pod) {
  // See k8s dashboard js logic
  // https://github.com/kubernetes/dashboard/blob/f63003113555ecf489b2a737797913a045b218c3/src/app/frontend/pod/list/card_component.js#L109
  let podStatus = pod.getIn(['status', 'phase']);
  let statusDisplay = podStatus;
  let reason = undefined;
  const statuses = pod.getIn(['status', 'containerStatuses']);
  if (statuses) {
    statuses.reverse().forEach(status => {
      const waiting = status.get('waiting');
      if (waiting) {
        podStatus = K8sPodDisplayStatusEnum.PENDING;
        reason = waiting.get('reason');
      }

      const terminated = status.get('terminated');
      if (terminated) {
        const terminatedSignal = terminated.get('signal');
        const terminatedExitCode = terminated.get('exitCode');
        const terminatedReason = terminated.get('reason');
        podStatus = K8sPodDisplayStatusEnum.TERMINATED;
        reason = terminatedReason;
        if (!reason) {
          if (terminatedSignal) {
            reason = `signal:${terminatedSignal}`;
          } else {
            reason = `exitCode:${terminatedExitCode}`;
          }
        }
      }
    });
  }

  if (podStatus === K8sPodDisplayStatusEnum.PENDING) {
    statusDisplay = reason ? `Waiting: ${reason}` : podStatus;
  }

  if (podStatus === K8sPodDisplayStatusEnum.TERMINATED) {
    statusDisplay = `Terminated: ${reason}`;
  }

  return {
    status: podStatus,
    statusDisplay: statusDisplay
  }
}