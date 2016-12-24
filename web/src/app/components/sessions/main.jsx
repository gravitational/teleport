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
var { fetchStoredSession } = require('app/modules/storedSessionsFilter/actions');
var {sessionsView} = require('app/modules/sessions/getters');
var {filter} = require('app/modules/storedSessionsFilter/getters');
var StoredSessionList = require('./storedSessionList.jsx');
var ActiveSessionList = require('./activeSessionList.jsx');
var Timer = require('./../timer.jsx');

var Sessions = React.createClass({
  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      data: sessionsView,
      storedSessionsFilter: filter
    }
  },

  refresh(){
    fetchStoredSession();    
  },

  render: function() {
    let {data, storedSessionsFilter} = this.state;
    return (
      <div className="grv-sessions grv-page">
        <Timer onTimeout={this.refresh} />
        <ActiveSessionList data={data} />
        <div className="m-t"/>
        <StoredSessionList data={data} filter={storedSessionsFilter}/>
      </div>
    );
  }
});

module.exports = Sessions;
