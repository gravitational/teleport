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

    //let sid = 'e0536e4c-0e1f-11e6-85fc-f0def19340e2';
    //let sid = '02aa3744-0e21-11e6-85fc-f0def19340e2';
///https://localhost:8080/web/sessions/195c1dd3-0e6c-11e6-8a80-f0def19340e2

    let sid = 'e64a8b03-0e6f-11e6-934b-f0def19340e2';
    api.get(`/v1/webapi/sites/-current-/sessions/${sid}/events`);
    api.get(`/v1/webapi/sites/-current-/sessions/${sid}/stream?offset=0&bytes=303`);

    let frm = new Date('12/12/2015').toISOString();
    let to = new Date('12/12/2016').toISOString();
    api.get(`/v1/webapi/sites/-current-/events?event=session.start&event=session.end&from=${frm}&to=${to}`);
    //api.get(`/v1/webapi/sites/-current-/events?from=${to}&to=${frm}`);
    //api.get(`/v1/webapi/sites/-current-/sessions/${sid}/stream?offset=0&bytes=303`);


    api.get(cfg.api.nodesPath).done((data=[])=>{
      var nodeArray = data.nodes.map(item=>item.node);
      reactor.dispatch(TLPT_NODES_RECEIVE, nodeArray);
    }).fail((err)=>{
      showError('Unable to retrieve list of nodes');
      logger.error('fetchNodes', err);
    })
  }
}
