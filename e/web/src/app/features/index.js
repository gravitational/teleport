// telebase imports
import { append, createSettings as create } from 'telebase-app/features/settings';

import OAuthFeature from './settingsAuth';
import TrustedClustersFeature from './settingsClusters';
import RolesFeature from './settingsRoles';

export const createSettings = routes => {  
  const settings = create(routes);
  append(settings, OAuthFeature);
  append(settings, RolesFeature)    
  append(settings, TrustedClustersFeature)    
  return settings;
}
