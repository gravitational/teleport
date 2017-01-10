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

const nodeHostNameByServerId = (serverId) => [ ['tlpt_nodes'], (nodeList) =>{
  let server = nodeList.find(item=> item.get('id') === serverId);
  return !server ? '' : server.get('hostname');
}];

const nodeListView = [['tlpt_nodes'], ['tlpt', 'siteId'], (nodeList, siteId) => {  
  nodeList = nodeList.filter(n => n.get('siteId') === siteId);  
  if (!nodeList) {
    return [];
  }

  return nodeList.map((item) => {
    var serverId = item.get('id');
    return {
      id: serverId,
      siteId: item.get('siteId'),
      hostname: item.get('hostname'),
      tags: getTags(item),
      addr: item.get('addr')
    }
  }).toJS();

}];

function getTags(node){
  var allLabels = [];
  var labels = node.get('labels');

  if(labels){
    labels.entrySeq().toArray().forEach(item=>{
      allLabels.push({
        role: item[0],
        value: item[1]
      });
    });
  }

  labels = node.get('cmd_labels');

  if(labels){
    labels.entrySeq().toArray().forEach(item=>{
      allLabels.push({
        role: item[0],
        value: item[1].get('result'),
        tooltip: item[1].get('command')
      });
    });
  }

  return allLabels;
}

export default {
  nodeListView,
  nodeHostNameByServerId
}
