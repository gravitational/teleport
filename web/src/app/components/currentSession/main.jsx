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
import { getters } from 'app/modules/currentSession/';
import SessionPlayer from './sessionPlayer.jsx';
import ActiveSession from './activeSession.jsx';
import cfg from 'app/config';
import { initSession } from 'app/modules/currentSession/actions';

const CurrentSessionHost = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      currentSession: getters.currentSession
    }
  },

  componentDidMount() {    
    setTimeout(() => initSession(this.props.params.sid), 0);    
  },

  render() {
    let { currentSession } = this.state;
    if(!currentSession){
      return null;
    }

    if(currentSession.active){
      return <ActiveSession {...currentSession}/>;
    }

    let { sid, siteId } = currentSession;
    let url = cfg.api.getFetchSessionUrl({ siteId, sid });
    
    return <SessionPlayer url={url}/>;
  }

});

export default CurrentSessionHost;
