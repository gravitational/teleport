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
import connect from './../connect';
import { fetchSiteEventsWithinTimeRange } from 'app/flux/storedSessionsFilter/actions';
import { storedSessionList, activeSessionList } from 'app/flux/sessions/getters';
import { filter } from 'app/flux/storedSessionsFilter/getters';
import appGetters from 'app/flux/app/getters';
import AjaxPoller from './../dataProvider.jsx';
import SessionList from './sessionList.jsx';
import { DocumentTitle } from './../documentTitle';
import withStorage from './../withStorage.jsx';

class Sessions extends React.Component {
    
  refresh = () => {
    return fetchSiteEventsWithinTimeRange();    
  }

  render() {            
    const { siteId, storedSessions, activeSessions, storedSessionsFilter } = this.props;
    const title = `${siteId} · Sessions`;
    return (      
      <DocumentTitle title={title}>
        <div className="grv-page grv-sessions">
          <SessionList
            storage={this.props.storage}
            activeSessions={activeSessions}
            storedSessions={storedSessions}
            filter={storedSessionsFilter}
          />
          <AjaxPoller onFetch={this.refresh} />
        </div>
      </DocumentTitle>  
    );
  }
}

function mapFluxToProps() {
  return {    
    siteId: appGetters.siteId,
    activeSessions: activeSessionList,
    storedSessions: storedSessionList,
    storedSessionsFilter: filter
  }
}

const SessionsWithStorage = withStorage(Sessions);

export default connect(mapFluxToProps)(SessionsWithStorage);