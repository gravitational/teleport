var React = require('react');
var {getters, actions} = require('app/modules/activeTerminal/');
var NotFoundPage = require('app/components/notFoundPage.jsx');
var SessionPlayer = require('./sessionPlayer.jsx');
var ActiveSession = require('./activeSession.jsx');

var CurrentSessionHost = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      activeSession: getters.activeSession
    }
  },

  componentDidMount(){
    var { sid } = this.props.params;
    if(!this.state.activeSession){
      actions.openSession(sid);
    }
  },

  render: function() {
    var activeSession = this.state.activeSession;
    if(!activeSession){
      return null;
    }

    if(activeSession.isNewSession || activeSession.active){
      return <ActiveSession activeSession={activeSession}/>;
    }
    
    return <SessionPlayer activeSession={activeSession}/>;
  }
});

module.exports = CurrentSessionHost;
