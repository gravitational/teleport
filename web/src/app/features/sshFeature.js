import cfg from 'app/config'
import Nodes from 'app/components/nodes/main';
import FeatureBase from './../featureBase';
import { addNavItem } from './../flux/app/actions';
import TerminalHost from '../components/terminal/terminalHost.jsx';

const sshRoutes = [
  {
    path: cfg.routes.nodes,
    title: "Nodes",
    component: Nodes
  }, {
    path: cfg.routes.terminal,
    title: "Terminal",
    components: {
      CurrentSessionHost: TerminalHost
    }
  }
]

const sshNavItem = {
  icon: 'fa fa-share-alt',
  to: cfg.routes.nodes,
  title: 'Nodes'
}

class SshFeature extends FeatureBase {

  constructor(routes) {        
    super();        
    routes.push(...sshRoutes);        
  }
  
  getIndexRoute(){
    return cfg.routes.nodes;
  }

  onload() {                  
    addNavItem(sshNavItem);
    //this.startProcessing();    
    //fetchAuthProviders().always(this.stopProcessing.bind(this))      
  }  
}

export default SshFeature;