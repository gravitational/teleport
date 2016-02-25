//var sort = require('app/common/sort');

const nodeListView = [ ['tlpt_nodes'], (nodes) =>{
    return nodes.valueSeq().map((item)=>{
      return {
        count: item.get('count'),
        ip: item.get('ip'),
        tags: ['tag1', 'tag2', 'tag3'],
        roles: ['r1', 'r2', 'r3']
      }
    }).toJS();
 }
];

export default {
  nodeListView
}
