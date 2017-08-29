import api from 'app/services/api';
import { ResourceEnum } from 'app/services/enums';
import cfg from 'app/config';

export function getOidc(){
  return api.get(cfg.getResourcesUrl(ResourceEnum.OIDC))
    .then(res => { return res.items || [] })   
}

export function getSaml(){
  return api.get(cfg.getResourcesUrl(ResourceEnum.SAML))
    .then(res => { return res.items || [] })   
}

export function getRoles(){
  return api.get(cfg.getResourcesUrl(ResourceEnum.ROLE))
    .then(res => { return res.items || [] })   
}

export function getTrustedClusters(){
  return api.get(cfg.getResourcesUrl(ResourceEnum.TRUSTED_CLUSTER))
    .then(res => { return res.items || [] })   
}

export function upsert(yaml){
  return api.put(cfg.getResourcesUrl(), { yaml } )          
    .then(res => { return res.items || [] })     
}

export function remove(kind, name){
  return api.delete(cfg.getRemoveResourceUrl(kind, name))        
}

