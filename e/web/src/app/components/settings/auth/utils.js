import { isString } from 'lodash';

export const roleEmptyPlaceHolder = {
  "claim": {
    "claim_value": ["role_name"]
  }
}

export function isValidOidcScope(name) {
  name = name || '';
  return name.length > 0;
}

export function toJSON(roleMapping) {
  roleMapping = roleMapping || [];
  let formatted = {};
  roleMapping.forEach( r => {
    formatted[r['claim']] = formatted[r['claim']] || {};
    formatted[r['claim']][r['value']] = r['roles'];
  })

  if (Object.getOwnPropertyNames(formatted).length === 0){
    return null;
  }
        
  return JSON.stringify(formatted, null, '\t');            
}

export function parseRoleMapping(str) {
  try {    
    let json = JSON.parse(str);
    let roleMapping = [];
    if (isValidRoleMapping(json)) {
      let claimNames = Object.getOwnPropertyNames(json);            
      claimNames.forEach(c => {        
        let claimValues = Object.getOwnPropertyNames(json[c]);
        claimValues.forEach(v => {
          roleMapping.push({
            claim: c,
            value: v,
            roles: json[c][v]
          });
        });                
      })            
    }

    return roleMapping;
  } catch (err) {
    return [];
  }
}

export function isValidRoleMapping(mapping) {
  let props = Object.getOwnPropertyNames(mapping);  
  let isValid = props.every(item => {
    return isValidRoleClaimMapping(mapping[item]);
  });

  return isValid;
}

export function isValidRoleClaimMapping(claim) {
  let props = Object.getOwnPropertyNames(claim);
  
  if (props.length === 0) {
    return false;
  }

  let isValid = props.every(item => {
    let valueArray = claim[item];
    let isValidArray = Array.isArray(valueArray) && valueArray.length > 0;    
    return isValidArray && valueArray.every(isString)
  });

  return isValid;    
}