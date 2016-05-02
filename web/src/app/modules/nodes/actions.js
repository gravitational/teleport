/*
Copyright 2015 Gravitational, Inc.

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

var reactor = require('app/reactor');
var { TLPT_NODES_RECEIVE }  = require('./actionTypes');
var api = require('app/services/api');
var cfg = require('app/config');
var {showError} = require('app/modules/notifications/actions');

const logger = require('app/common/logger').create('Modules/Nodes');

export default {
  fetchNodes(){
    api.get(cfg.api.nodesPath).done((data=[])=>{
      var nodeArray = data.nodes.map(item=>item.node);
      reactor.dispatch(TLPT_NODES_RECEIVE, nodeArray);
    }).fail((err)=>{
      showError('Unable to retrieve list of nodes');
      logger.error('fetchNodes', err);
    })
  }
}
