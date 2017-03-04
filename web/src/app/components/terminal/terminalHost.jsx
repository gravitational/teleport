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

import React from 'react';
import reactor from 'app/reactor';
import cfg from 'app/config';
import SessionLeftPanel from './terminalHostPanel';
import session from 'app/services/session';
import Terminal from 'app/common/term/terminal';
import getters from 'app/modules/terminal/getters';
import { updateSessionFromEventStream } from 'app/modules/terminal/actions';
import Indicator from './../indicator.jsx';
import { initTerminal } from 'app/modules/terminal/actions';

const TerminalHost = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      store: getters.store
    }
  },

  componentDidMount() {    
    setTimeout(() => initTerminal(this.props.routeParams), 0);    
  },  

  render() {
    let { store } = this.state;        
    let serverLabel = store.getServerLabel();
    let { parties = [], status, ...props } = store.toJS();
    
    let $content = null;
    
    if (status.isReady) {
      $content = (<TtyTerminal {...props}/>)
    } 
    
    if (status.isLoading) {
      $content = (<Indicator type="bounce" />);
    }

    if (status.isError) {
      $content = (<ErrorIndicator text={status.errorText} />);
    }

    if (status.isNotFound) {
      $content = (<SidNotFoundError />);
    }
            
    return (
     <div className="grv-current-session">
       <SessionLeftPanel parties={parties}/>
       <div className="grv-current-session-server-info">
         <h3>{serverLabel}</h3>
      </div>
      {$content}               
     </div>
    );
  }
});

const TtyTerminal = React.createClass({
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
    return ( <div ref="container"/> );
  }
});

const ErrorIndicator = ({ text }) => (
  <div className="grv-terminalhost-indicator-error">
    <i className="fa fa-exclamation-triangle fa-3x text-warning"></i>
    <div className="m-l">
      <strong>Connection error</strong>
      <div className="text-center"><small>{text}</small></div>
    </div>
  </div>
)

const SidNotFoundError = () => (
  <div className="grv-terminalhost-indicator-error">    
    <div className="text-center">
      <strong>The session is no longer active</strong>    
      <div className="m-t">
        <button className="btn btn-sm btn-primary m-r"> Start New </button>        
        <button className="btn btn-sm btn-primary"> Replay </button>
      </div>
    </div>
  </div>
)

export default TerminalHost;
