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
import { fetchSiteEventsWithinTimeRange } from 'app/flux/storedSessionsFilter/actions';
import { storedSessionList, activeSessionList } from 'app/flux/sessions/getters';
import { filter } from 'app/flux/storedSessionsFilter/getters';
import AjaxPoller from './../dataProvider.jsx';
import SessionList from './sessionList.jsx';

const Sessions = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      activeSessions: activeSessionList,
      storedSessions: storedSessionList,
      storedSessionsFilter: filter
    }
  },

  refresh(){
    return fetchSiteEventsWithinTimeRange();    
  },

  render() {    
    const { storedSessions, activeSessions, storedSessionsFilter } = this.state;
    return (      
      <div className="grv-page grv-sessions">                                      
        <SessionList
          activeSessions={activeSessions}
          storedSessions={storedSessions}
          filter={storedSessionsFilter}
        />
        <AjaxPoller onFetch={this.refresh} />        
      </div>        
    );
  }
});

export default Sessions;
