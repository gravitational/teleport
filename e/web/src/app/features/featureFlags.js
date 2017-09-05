
import { getAcl } from 'telebase-app/flux/userAcl/store';

export function isTrustedClrsEnabled(){  
  return getAcl().getClusterAccess().read;      
}

export function isRolesEnabled(){  
  return getAcl().getRoleAccess().read;      
}

export function isAuthConnectorsEnabled(){  
  return getAcl().getConnectorAccess().read;      
}

export function isAnythingEnabled(){
  return isTrustedClrsEnabled() || isRolesEnabled() || isAuthConnectorsEnabled();
}