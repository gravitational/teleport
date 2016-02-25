//var sort = require('app/common/sort');

const nodeListView = [ ['tlpt_nodes'], (nodes) =>{
    return nodes.map((item)=>{
      return {
        tags: getTags(item),
        ip: item.get('addr')
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
