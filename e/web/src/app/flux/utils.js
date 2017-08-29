import yaml2Js from 'js-yaml';
import { isObject } from 'lodash';

export function checkResourceKind(kinds, yaml, errMsg){
  // do not try to handle yaml exception, instead try to extract the kind information only
  // let the backend handle yaml syntax too.
  let json = null;
  try{
    json = yaml2Js.safeLoad(yaml);    
  }catch(err) {
    return
  }
  
  errMsg = errMsg || 'missing valid kind value for given resource type';
  if (isObject(json) && !kinds.some( k => k === json.kind )){
    throw Error(errMsg);      
  }     
}