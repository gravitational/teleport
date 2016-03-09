var React = require('react');
var {getters, actions} = require('app/modules/activeTerminal/');
var Tty = require('app/common/tty');
var TtyTerminal = require('./../terminal.jsx');
var EventStreamer = require('./eventStreamer.jsx');
var SessionLeftPanel = require('./sessionLeftPanel');
var {showSelectNodeDialog, closeSelectNodeDialog} = require('app/modules/dialogs/actions');

var ActiveSession = React.createClass({

  componentWillUnmount(){
    closeSelectNodeDialog();
  },

  render: function() {
    var {serverIp} = this.props.activeSession;
    return (
     <div className="grv-current-session">
       <SessionLeftPanel/>
       <div>
         <div className="grv-current-session-server-info">
           <h3>{serverIp}<span className="btn label label-primary" onClick={showSelectNodeDialog} >Change node</span>
           </h3>
         </div>
       </div>
       <TtyConnection {...this.props.activeSession} />
     </div>
     );
  }
});

var TtyConnection = React.createClass({

  getInitialState() {
    this.tty = new Tty(this.props)
    this.tty.on('open', ()=> this.setState({ isConnected: true }));
    return {isConnected: false};
  },

  componentWillUnmount() {
    this.tty.disconnect();
  },

  componentWillReceiveProps(nextProps){
    if(nextProps.serverId !== this.props.serverId ||
      nextProps.login !== this.props.login){
        this.tty.reconnect(nextProps);        
      }
  },

  render() {
    return (
      <div style={{height: '100%'}}>
        <TtyTerminal tty={this.tty} cols={this.props.cols} rows={this.props.rows} />
        { this.state.isConnected ? <EventStreamer sid={this.props.sid}/> : null }
      </div>
    )
  }
});

module.exports = ActiveSession;
