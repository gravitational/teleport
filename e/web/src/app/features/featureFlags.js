
import { getAcl } from 'telebase-app/flux/userAcl/store';

export function isTrustedClrsEnabled(){  
  return getAcl().getClusterAccess().list;      
}

export function isRolesEnabled(){  
  return getAcl().getRoleAccess().list;      
}

export function isAuthConnectorsEnabled(){  
  return getAcl().getConnectorAccess().list;      
}

export function isAnythingEnabled(){
  return isTrustedClrsEnabled() || isRolesEnabled() || isAuthConnectorsEnabled();
}