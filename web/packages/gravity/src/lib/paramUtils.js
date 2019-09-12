/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*eslint no-useless-escape: "off"*/

import Logger from 'shared/libs/logger';
import IpSubnetCalculator from 'ip-subnet-calculator';
const logger = Logger.create('paramUtils');

export function isDomainName(value){
  return /^((?=.{1,255}$)[0-9A-Za-z](?:(?:[0-9A-Za-z]|\b-){0,61}[0-9A-Za-z])?(?:\.[0-9A-Za-z](?:(?:[0-9A-Za-z]|\b-){0,61}[0-9A-Za-z])?)*\.?)$/i.test(value);
}

export function isValidIp4(value){
  return /^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))$/i.test(value);
}

export function isValidPort(value){
  return /^[0-9]{1,5}$/i.test(value);
}

export function parseWebConfig(jsonStr) {
  jsonStr = jsonStr || '{}';
  let webConfig = {};

  try {
    jsonStr = jsonStr.replace(/\n/g, " ").replace(/\t/g, " ");
    webConfig = JSON.parse(jsonStr)
  } catch (err) {
    logger.error('parseWebConfig', err);
  }

  return webConfig;
}

export function parseCidr(cidr) {
  cidr = cidr || '';
  let [ip, prefix] = cidr.split('/');
  let isNumber = /^\d+$/.test(prefix);
  if (!isValidIp4(ip) || !isNumber) {
    return null;
  }

  prefix = new Number(prefix);
  if (prefix < 1 || prefix > 32) {
    return null;
  }

  let result = IpSubnetCalculator.calculateSubnetMask(ip, prefix);
  if (!result) {
    return null;
  }

  let totalHost = result.ipHigh - result.ipLow;

  return {
    totalHost
  }
}