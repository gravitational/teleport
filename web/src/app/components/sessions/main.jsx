var React = require('react');
var reactor = require('app/reactor');
var {getters, actions} = require('app/modules/sessions');
var StoredSessionList = require('./storedSessionList.jsx');
var ActiveSessionList = require('./activeSessionList.jsx');

var Sessions = React.createClass({
  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {data: getters.sessionsView}
  },

  render: function() {
    let {data} = this.state;
    return (
      <div className="grv-sessions grv-page">
        <ActiveSessionList data={data}/>
        <hr className="grv-divider"/>
        <StoredSessionList data={data}/>
      </div>
    );
  }
});

module.exports = Sessions;
