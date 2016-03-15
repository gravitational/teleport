var React = require('react');
var reactor = require('app/reactor');
var {getters, actions} = require('app/modules/activeTerminal/');
var SessionPlayer = require('./sessionPlayer.jsx');
var ActiveSession = require('./activeSession.jsx');

var CurrentSessionHost = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      currentSession: getters.activeSession
    }
  },

  componentDidMount(){
    var { sid } = this.props.params;
    if(!this.state.currentSession){
      actions.openSession(sid);
    }
  },

  render: function() {
    var currentSession = this.state.currentSession;
    if(!currentSession){
      return null;
    }

    if(currentSession.isNewSession || currentSession.active){
      return <ActiveSession activeSession={currentSession}/>;
    }

    return <SessionPlayer activeSession={currentSession}/>;
  }
});

module.exports = CurrentSessionHost;
