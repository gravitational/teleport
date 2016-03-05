var reactor = require('app/reactor');
var {sessionsByServer} = require('./../sessions/getters');

const nodeListView = [ ['tlpt_nodes'], (nodes) =>{
    return nodes.map((item)=>{
      var serverId = item.get('id');
      var sessions = reactor.evaluate(sessionsByServer(serverId));
      return {
        id: serverId,
        hostname: item.get('hostname'),
        tags: getTags(item),
        addr: item.get('addr'),
        sessionCount: sessions.size
      }
    }).toJS();
 }
];

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
  nodeListView
}
