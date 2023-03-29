/**
 * Copyright 2023 Gravitational, Inc.
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

import { ResourceKind } from '../Shared/ResourceKind';

import type { DiscoverEventResource } from 'teleport/services/userEvent';

export enum DatabaseLocation {
  Aws,
  SelfHosted,
  Gcp,
  Azure,
  Microsoft,

  TODO,
}

// DatabaseEngine represents the db "protocol".
export enum DatabaseEngine {
  Postgres,
  MySql,
  MongoDb,
  Redis,
  CoackroachDb,
  SqlServer,
  Snowflake,
  Cassandra,
  ElasticSearch,
  DynamoDb,
  Redshift,
}

export interface ResourceSpec {
  dbMeta?: { location: DatabaseLocation; engine: DatabaseEngine };
  name: string;
  popular?: boolean;
  kind: ResourceKind;
  Icon: React.ReactElement;
  // keywords are filter words that user may use to search for
  // this resource.
  keywords: string;
  // hasAccess is a flag to mean that user has
  // the preliminary permissions to add this resource.
  hasAccess?: boolean;
  // unguidedLink is the link out to this resources documentation.
  // It is used as a flag, that when defined, means that
  // this resource is not "guided" (has no UI interactive flow).
  unguidedLink?: string;
  // event is the expected backend enum event name that describes
  // the type of this resource (eg. server v. kubernetes),
  // used for usage reporting.
  event: DiscoverEventResource;
}
