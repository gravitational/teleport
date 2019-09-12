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

import reactor from 'gravity/reactor';
import * as actionTypes from './actionTypes';
import k8s from 'gravity/cluster/services/k8s';

export function fetchDeployments(namespace){
  return k8s.getDeployments(namespace).done(jsonArray=>{
    reactor.dispatch(actionTypes.RECEIVE_DEPLOYMENTS, jsonArray);
  });
}

export function fetchDaemonSets(namespace){
  return k8s.getDaemonSets(namespace).done(jsonArray=>{
    reactor.dispatch(actionTypes.RECEIVE_DAEMONSETS, jsonArray);
  });
}

export function fetchJobs(namespace){
  return k8s.getJobs(namespace).done(jsonArray=>{
    reactor.dispatch(actionTypes.RECEIVE_JOBS, jsonArray);
  });
}