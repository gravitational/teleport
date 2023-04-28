/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import nodeAccessRO from './nodeAccessRO.yaml?raw';
import nodeAccessRW from './nodeAccessRW.yaml?raw';
import connDiagRW from './connDiagRW.yaml?raw';
import kubeAccessRW from './kubeAccessRW.yaml?raw';
import kubeAccessRO from './kubeAccessRO.yaml?raw';
import dbAccessRW from './dbAccessRW.yaml?raw';
import dbAccessRO from './dbAccessRO.yaml?raw';
import dbCU from './dbCU.yaml?raw';
import integrationRWEAndDbCU from './integrationRWEAndDbCU.yaml?raw';

export {
  nodeAccessRO,
  nodeAccessRW,
  connDiagRW,
  kubeAccessRW,
  kubeAccessRO,
  dbAccessRO,
  dbAccessRW,
  dbCU,
  integrationRWEAndDbCU,
};
