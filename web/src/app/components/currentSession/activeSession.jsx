var React = require('react');
var {getters, actions} = require('app/modules/activeTerminal/');
var Tty = require('app/common/tty');
var TtyTerminal = require('./../terminal.jsx');
var EventStreamer = require('./eventStreamer.jsx');
var SessionLeftPanel = require('./sessionLeftPanel');
var {showSelectNodeDialog, closeSelectNodeDialog} = require('app/modules/dialogs/actions');
var SelectNodeDialog = require('./../selectNodeDialog.jsx');

var ActiveSession = React.createClass({

  componentWillUnmount(){
    closeSelectNodeDialog();
  },

  render: function() {
    let {serverIp, login, parties} = this.props.activeSession;
    let serverLabelText = `${login}@${serverIp}`;

    if(!serverIp){
      serverLabelText = '';
    }

    return (
     <div className="grv-current-session">
       <SessionLeftPanel parties={parties}/>
       <div className="grv-current-session-server-info">      
         <h3>{serverLabelText}</h3>
       </div>
       <TtyConnection {...this.props.activeSession} />
     </div>
     );
  }
});

var TtyConnection = React.createClass({

  getInitialState() {
    this.tty = new Tty(this.props)
    this.tty.on('open', ()=> this.setState({ ...this.state, isConnected: true }));

    var {serverId, login} = this.props;
    return {serverId, login, isConnected: false};
  },

  componentDidMount(){
    // temporary hack
    SelectNodeDialog.onServerChangeCallBack = this.componentWillReceiveProps.bind(this);
  },

  componentWillUnmount() {
    SelectNodeDialog.onServerChangeCallBack = null;
    this.tty.disconnect();
  },

  componentWillReceiveProps(nextProps){
    var {serverId} = nextProps;
    if(serverId && serverId !== this.state.serverId){
      this.tty.reconnect({serverId});
      this.refs.ttyCmntInstance.term.focus();
      this.setState({...this.state, serverId });
    }
  },

  render() {
    return (
      <div style={{height: '100%'}}>
        <TtyTerminal ref="ttyCmntInstance" tty={this.tty} cols={this.props.cols} rows={this.props.rows} />
        { this.state.isConnected ? <EventStreamer sid={this.props.sid}/> : null }
      </div>
    )
  }
});

module.exports = ActiveSession;
