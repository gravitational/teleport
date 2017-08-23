import cfg from '../config';
import Settings from '../components/settings/main'
import FeatureActivator from './../featureActivator';
import { initSettings } from './../flux/settings/actions'
import OAuthFeature from './oauthFeature';
import TrustedClustersFeature from './trustedClustersFeature';
import RolesFeature from './rolesFeature';

const featureActivator = new FeatureActivator();
const featureRoutes = []
const features = [   
  new OAuthFeature(featureRoutes), 
  new TrustedClustersFeature(featureRoutes),
  new RolesFeature(featureRoutes), 
]

features.forEach( f => featureActivator.register(f));

const onEnterIndex = (localtion, replace) => {      
  replace(features[0].getIndexRoute()) 
}

const onEnter = () => {       
  initSettings(featureActivator);  
}
  
const routes = [
  { path: cfg.routes.settingsBase, onEnter: onEnterIndex },
  {
    path: cfg.routes.settingsBase,
    title: 'Settings',
    onEnter: onEnter,
    component: Settings,  
    childRoutes: featureRoutes              
}]

export default routes;