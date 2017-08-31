import cfg from 'app/config'
import FeatureBase from './../featureBase';
import { addNavItem } from './../flux/app/actions';
import Sessions from '../components/sessions/main.jsx';
import PlayerHost from '../components/player/playerHost.jsx';

const auditRoutes = [
  {
    path: cfg.routes.sessions,
    title: "Stored Sessions",
    component: Sessions
  }, {
    path: cfg.routes.player,
    title: "Player",
    components: {
      CurrentSessionHost: PlayerHost
    }
  }
]

const auditNavItem = {
  icon: 'fa fa-share-alt',
  to: cfg.routes.nodes,
  title: 'Nodes'
}

class AuditFeature extends FeatureBase {

  constructor(routes) {        
    super();        
    routes.push(...auditRoutes);        
  }
  
  getIndexRoute(){
    return cfg.routes.nodes;
  }

  onload() {                  
    addNavItem(auditNavItem);
    //this.startProcessing();    
    //fetchAuthProviders().always(this.stopProcessing.bind(this))      
  }  
}

export default AuditFeature;