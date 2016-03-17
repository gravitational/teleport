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
var {getters, actions} = require('app/modules/currentSession/');
var SessionPlayer = require('./sessionPlayer.jsx');
var ActiveSession = require('./activeSession.jsx');

var CurrentSessionHost = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      currentSession: getters.currentSession
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
      return <ActiveSession session={currentSession}/>;
    }

    return <SessionPlayer session={currentSession}/>;
  }
});

module.exports = CurrentSessionHost;
