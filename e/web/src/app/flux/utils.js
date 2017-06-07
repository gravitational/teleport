// FIXME: a temporary fix for #1630 
export function isHiddenRoleName(name) {
  name = name || '';  
  return name.indexOf('ca:') === 0;
}