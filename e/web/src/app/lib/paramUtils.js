import expiry from 'expiry-js';
import moment from 'moment';

export function isValidRoleName(name) {  
  return /^\w*$/.test(name);
}

export function isValidLoginName(name) {  
  return /^[a-z0-9_-]{1,32}$/.test(name);
}

export function isValidTtlDuration(str) {
  expiry(str);
  return false;
}

export function parseDuration(str) {    
  // handle pure numbers without any characters like m,h
  if (!isNaN(Number(str))) {
    throw Error('Invalid duration string');
  }

  // quick way to limit number of characters allowed by expiry  
  str = str || '';
  str = str.toLowerCase();
  expiry(str)  
  return expiry(str).asMilliseconds();
}

export function durationToStr(ms) {  
  let duration = moment.duration(ms);
  let h = Math.round(duration.asHours());
  let m = Math.round(duration.minutes());
  
  let str = '0h0m'  
  if (h > 0) {
    str = `${h}h`
  }

  if (m < 0) {
    str = `${str}${m}`
  }
  
  return str;
}

const ALL_SPACES_REGX = / /gi;

export function parseNodeLabel(str) {  
  let nodeLabels = {};
  str = str || '';  
  str.replace(ALL_SPACES_REGX, '').split(',').forEach(item => {
    let [key, value] = item.split('=');

    if ((!key || key.length === 0) || (!value || value.length === 0)) {
      throw Error('Cannot parse labels');
    }
    
    nodeLabels[key] = value;
  })

  return nodeLabels;
}