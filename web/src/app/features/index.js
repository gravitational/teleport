import cfg from '../config';
import FeatureActivator from './../featureActivator';
import SshFeature from './sshFeature';
import AuditFeature from './auditFeature';
import { initApp } from '../flux/app/actions';

const featureActivator = new FeatureActivator();
const featureRoutes = []
const features = [   
  new SshFeature(featureRoutes),   
  new AuditFeature(featureRoutes)
]

features.forEach(f => featureActivator.register(f));
  
const routes = [
  { 
      path: cfg.routes.app, 
      onEnter: initApp,
      childRoutes: [
        ...featureRoutes,                    
      ]
  }
]

export default routes;