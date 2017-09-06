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