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
var NavLeftBar = require('./navLeftBar');
var reactor = require('app/reactor');
var { getters } = require('app/modules/app');
var { initApp, checkIfValidUser } = require('app/modules/app/actions');
var NotificationHost = require('./notificationHost.jsx');
var Timer = require('./timer.jsx');

var App = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      app: getters.appState
    }
  },

  componentWillMount(){
    initApp();
  },

  render() {
    if(this.state.app.isInitializing){
      return null;
    }

    return (
      <div className="grv-tlpt grv-flex grv-flex-row">
        <Timer onTimeout={checkIfValidUser} interval={4000}/>
        <NotificationHost/>
        {this.props.CurrentSessionHost}
        <NavLeftBar/>
        {this.props.children}
      </div>
    );
  }
})

module.exports = App;
