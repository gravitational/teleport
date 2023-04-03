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

import { DiscoverEventResource } from 'teleport/services/userEvent';

import {
  DatabaseEngine,
  DatabaseLocation,
  ResourceSpec,
} from '../SelectResource/types';

import type { JoinRole } from 'teleport/services/joinToken';

export enum ResourceKind {
  Application,
  Database,
  Desktop,
  Kubernetes,
  Server,
}

export function resourceKindToJoinRole(kind: ResourceKind): JoinRole {
  switch (kind) {
    case ResourceKind.Application:
      return 'App';
    case ResourceKind.Database:
      return 'Db';
    case ResourceKind.Desktop:
      return 'WindowsDesktop';
    case ResourceKind.Kubernetes:
      return 'Kube';
    case ResourceKind.Server:
      return 'Node';
  }
}

export function resourceKindToEventResource(
  resourceSpec: ResourceSpec
): DiscoverEventResource {
  switch (resourceSpec.kind) {
    case ResourceKind.Database:
      const { engine, location } = resourceSpec.dbMeta;
      if (location === DatabaseLocation.AWS) {
        if (engine === DatabaseEngine.PostgreSQL) {
          return DiscoverEventResource.DatabasePostgresRds;
        }
        if (engine === DatabaseEngine.MySQL) {
          return DiscoverEventResource.DatabaseMysqlRds;
        }
        if (engine === DatabaseEngine.SQLServer) {
          return DiscoverEventResource.DatabaseSqlServerRds;
        }
        if (engine === DatabaseEngine.RedShift) {
          return DiscoverEventResource.DatabasePostgresRedshift;
        }
      }

      if (location === DatabaseLocation.SelfHosted) {
        if (engine === DatabaseEngine.PostgreSQL) {
          return DiscoverEventResource.DatabasePostgresSelfHosted;
        }
        if (engine === DatabaseEngine.MySQL) {
          return DiscoverEventResource.DatabaseMysqlSelfHosted;
        }
        if (engine === DatabaseEngine.Mongo) {
          return DiscoverEventResource.DatabaseMongodbSelfHosted;
        }
        if (engine === DatabaseEngine.SQLServer) {
          return DiscoverEventResource.DatabaseSqlServerSelfHosted;
        }
        if (engine === DatabaseEngine.Redis) {
          return DiscoverEventResource.DatabaseRedisSelfHosted;
        }
      }

      if (location === DatabaseLocation.GCP) {
        if (engine === DatabaseEngine.PostgreSQL) {
          return DiscoverEventResource.DatabasePostgresGcp;
        }
        if (engine === DatabaseEngine.MySQL) {
          return DiscoverEventResource.DatabaseMysqlGcp;
        }
        if (engine === DatabaseEngine.SQLServer) {
          return DiscoverEventResource.DatabaseMysqlGcp;
        }
      }
      console.error(`resource database event not defined for ${resourceSpec}`);
      return null;
    case ResourceKind.Desktop:
      return DiscoverEventResource.WindowsDesktop;
    case ResourceKind.Kubernetes:
      return DiscoverEventResource.Kubernetes;
    case ResourceKind.Server:
      return DiscoverEventResource.Server;
    case ResourceKind.Application:
      return DiscoverEventResource.ApplicationHttp;
    default:
      console.error(`resource event not defined for ${resourceSpec}`);
  }
}
