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

import { Participant } from 'teleport/services/session';

interface DocumentBase {
  id?: number;
  title?: string;
  clusterId?: string;
  url: string;
  kind: 'terminal' | 'nodes' | 'blank';
  created: Date;
}

export interface DocumentBlank extends DocumentBase {
  kind: 'blank';
}

export interface DocumentSsh extends DocumentBase {
  status: 'connected' | 'disconnected';
  kind: 'terminal';
  sid?: string;
  serverId: string;
  login: string;
}

export interface DocumentNodes extends DocumentBase {
  kind: 'nodes';
}

export type Document = DocumentNodes | DocumentSsh | DocumentBlank;

export type Parties = Record<string, Participant[]>;
