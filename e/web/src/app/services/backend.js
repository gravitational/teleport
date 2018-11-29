import api from '../services/api';
import { ResourceEnum } from '../services/enums';
import cfg from '../config';

const unpackItems = res => res.items || [];

export function getAuthProviders(){
  return api.get(cfg.getResourcesUrl(ResourceEnum.AUTH_CONNECTORS))
    .then(unpackItems)
}

export function getRoles(){
  return api.get(cfg.getResourcesUrl(ResourceEnum.ROLE))
    .then(unpackItems)
}

export function getTrustedClusters(){
  return api.get(cfg.getResourcesUrl(ResourceEnum.TRUSTED_CLUSTER))
    .then(unpackItems)
}

export function upsert(kind, yaml, isNew=false){
  const req = { kind, content: yaml };
  if(isNew){
    return api.post(cfg.getResourcesUrl(), req).then(unpackItems)
  }

  return api.put(cfg.getResourcesUrl(), req).then(unpackItems)
}

export function remove(kind, name){
  return api.delete(cfg.getRemoveResourceUrl(kind, name))
}

export function fetchLicenseStatus(){
  return api.get(cfg.api.licenseStatusPath);
}

export const getErrorText = api.getErrorText;