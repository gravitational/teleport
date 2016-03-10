var React = require('react');
var reactor = require('app/reactor');
var {getters} = require('app/modules/sessions');
var SessionList = require('./sessionList.jsx');

var Sessions = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      sessionsView: getters.sessionsView
    }
  },

  render: function() {
    var data = this.state.sessionsView;
    return ( <SessionList sessionRecords={data}/> );
  }

});

module.exports = Sessions;
