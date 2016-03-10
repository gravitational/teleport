var React = require('react');
var reactor = require('app/reactor');
var userGetters = require('app/modules/user/getters');
var nodeGetters = require('app/modules/nodes/getters');
var NodeList = require('./nodeList.jsx');

var Nodes = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      nodeRecords: nodeGetters.nodeListView,
      user: userGetters.user
    }
  },

  render: function() {
    var nodeRecords = this.state.nodeRecords;
    var logins = this.state.user.logins;
    return ( <NodeList nodeRecords={nodeRecords} logins={logins}/> );
  }
});

module.exports = Nodes;
