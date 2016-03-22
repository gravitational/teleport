/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

var React = require('react');
var reactor = require('app/reactor');
var {nodeHostNameByServerId} = require('app/modules/nodes/getters');
var Tty = require('app/common/tty');
var TtyTerminal = require('./../terminal.jsx');
var EventStreamer = require('./eventStreamer.jsx');
var SessionLeftPanel = require('./sessionLeftPanel');
var {closeSelectNodeDialog} = require('app/modules/dialogs/actions');
var SelectNodeDialog = require('./../selectNodeDialog.jsx');

var ActiveSession = React.createClass({

  getInitialState() {
    this.tty = new Tty(this.props)
    this.tty.on('open', ()=> this.setState({ ...this.state, isConnected: true }));

    var {serverId, login} = this.props;
    return {serverId, login, isConnected: false};
  },

  componentDidMount(){
    SelectNodeDialog.onServerChangeCallBack = this.componentWillReceiveProps.bind(this);
  },

  componentWillUnmount() {
    closeSelectNodeDialog();
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

  render: function() {
    let {login, parties, serverId} = this.props;
    let serverLabelText = '';
    if(serverId){
      let hostname = reactor.evaluate(nodeHostNameByServerId(serverId));
      serverLabelText = `${login}@${hostname}`;
    }

    return (
     <div className="grv-current-session">
       <SessionLeftPanel parties={parties}/>
       <div className="grv-current-session-server-info">
         <h3>{serverLabelText}</h3>
       </div>
       <div style={{height: '100%'}}>
         <TtyTerminal ref="ttyCmntInstance" tty={this.tty} cols={this.props.cols} rows={this.props.rows} />
         { this.state.isConnected ? <EventStreamer sid={this.props.sid}/> : null }
       </div>
     </div>
     );
  }
});

module.exports = ActiveSession;
