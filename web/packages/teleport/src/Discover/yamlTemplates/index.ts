/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import awsAppAccessRO from './awsAppAccessRO.yaml?raw';
import awsAppAccessRW from './awsAppAccessRW.yaml?raw';
import connDiagRW from './connDiagRW.yaml?raw';
import dbAccessRO from './dbAccessRO.yaml?raw';
import dbAccessRW from './dbAccessRW.yaml?raw';
import dbCU from './dbCU.yaml?raw';
import integrationAndAppRW from './integrationAndAppRW.yaml?raw';
import integrationRWE from './integrationRWE.yaml?raw';
import integrationRWEAndDbCU from './integrationRWEAndDbCU.yaml?raw';
import integrationRWEAndNodeRWE from './integrationRWEAndNodeRWE.yaml?raw';
import kubeAccessRO from './kubeAccessRO.yaml?raw';
import kubeAccessRW from './kubeAccessRW.yaml?raw';
import nodeAccessRO from './nodeAccessRO.yaml?raw';
import nodeAccessRW from './nodeAccessRW.yaml?raw';

export {
  nodeAccessRO,
  nodeAccessRW,
  connDiagRW,
  kubeAccessRW,
  kubeAccessRO,
  dbAccessRO,
  dbAccessRW,
  dbCU,
  integrationRWE,
  integrationRWEAndDbCU,
  integrationRWEAndNodeRWE,
  integrationAndAppRW,
  awsAppAccessRW,
  awsAppAccessRO,
};
