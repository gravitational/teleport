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
var SessionLeftPanel = require('./sessionLeftPanel');
var cfg = require('app/config');
var session = require('app/services/session');
var Terminal = require('app/common/term/terminal');
var {updateSessionFromEventStream} = require('app/modules/currentSession/actions');

var ActiveSession = React.createClass({
  render() {
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
       <TtyTerminal ref="ttyCmntInstance" {...this.props} />
     </div>
     );
  }
});

var TtyTerminal = React.createClass({
  componentDidMount() {
    let { serverId, siteId, login, sid } = this.props;
    let {token} = session.getUserData();
    let url = cfg.api.getSiteUrl(siteId);

    let options = {
      tty: {
        serverId, login, sid, token, url
      },     
     el: this.refs.container
    }

    this.terminal = new Terminal(options);
    this.terminal.ttyEvents.on('data', updateSessionFromEventStream(siteId));
    this.terminal.open();
  },

  componentWillUnmount() {
    this.terminal.destroy();
  },

  shouldComponentUpdate() {
    return false;
  },

  render() {
    return ( <div ref="container">  </div> );
  }
});

module.exports = ActiveSession;
