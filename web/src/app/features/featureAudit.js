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

import cfg from 'app/config'
import FeatureBase from './../featureBase';
import { addNavItem } from './../flux/app/actions';
import Sessions from '../components/sessions/main.jsx';
import PlayerHost from '../components/player/playerHost.jsx';
import reactor from 'app/reactor';
import { fetchSiteEventsWithinTimeRange } from 'app/flux/storedSessionsFilter/actions';
import { getAcl } from '../flux/userAcl/store';

const auditNavItem = {
  icon: 'fa  fa-group',
  to: cfg.routes.sessions,
  title: 'Sessions'
}

class AuditFeature extends FeatureBase {
    
  componentDidMount() {    
    this.init()    
  }

  init() {
    if (!this.wasInitialized()) {      
      reactor.batch(() => {
        this.startProcessing();
        fetchSiteEventsWithinTimeRange()
          .done(this.stopProcessing.bind(this))
          .fail(this.handleError.bind(this))                                                  
      })      
    }                
  }

  constructor(routes) {        
    super();        
    const auditRoutes = [
      {
        path: cfg.routes.sessions,
        title: "Stored Sessions",
        component: this.withMe(Sessions)
      }, {
        path: cfg.routes.player,
        title: "Player",
        components: {
          CurrentSessionHost: PlayerHost
        }
      }
    ];

    routes.push(...auditRoutes);        
  }
      
  onload() {     
    const sessAccess = getAcl().getSessionAccess();    
    if (sessAccess.list) {
      addNavItem(auditNavItem);  
      this.init();
    }        
  }  
}

export default AuditFeature;