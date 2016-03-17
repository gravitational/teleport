var React = require('react');
var reactor = require('app/reactor');
var {sessionsView} = require('app/modules/sessions/getters');
var {filter} = require('app/modules/storedSessionsFilter/getters');
var StoredSessionList = require('./storedSessionList.jsx');
var ActiveSessionList = require('./activeSessionList.jsx');

var Sessions = React.createClass({
  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      data: sessionsView,
      storedSessionsFilter: filter
    }
  },

  render: function() {
    let {data, storedSessionsFilter} = this.state;
    return (
      <div className="grv-sessions grv-page">
        <ActiveSessionList data={data}/>
        <hr className="grv-divider"/>
        <StoredSessionList data={data} filter={storedSessionsFilter}/>
      </div>
    );
  }
});

module.exports = Sessions;
