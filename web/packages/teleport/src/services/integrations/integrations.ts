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

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import { TaskState } from 'teleport/Integrations/status/AwsOidc/Tasks/constants';
import api from 'teleport/services/api';

import { App } from '../apps';
import makeApp from '../apps/makeApps';
import auth, { MfaChallengeScope } from '../auth/auth';
import {
  withGenericUnsupportedError,
  withUnsupportedLabelFeatureErrorConversion,
} from '../version/unsupported';
import {
  AwsDatabaseVpcsResponse,
  AwsOidcDeployDatabaseServicesRequest,
  AWSOIDCDeployedDatabaseService,
  AwsOidcDeployServiceRequest,
  AwsOidcListDatabasesRequest,
  AWSOIDCListDeployedDatabaseServiceResponse,
  AwsOidcPingRequest,
  AwsOidcPingResponse,
  AwsRdsDatabase,
  CreateAwsAppAccessRequest,
  EnrollEksClustersRequest,
  EnrollEksClustersResponse,
  ExportedIntegrationCaResponse,
  Integration,
  IntegrationCreateRequest,
  IntegrationCreateResult,
  IntegrationDeleteRequest,
  IntegrationDiscoveryRules,
  IntegrationKind,
  IntegrationListResponse,
  IntegrationStatusCode,
  IntegrationUpdateRequest,
  IntegrationUpdateResult,
  IntegrationWithSummary,
  ListAwsRdsDatabaseResponse,
  ListAwsRdsFromAllEnginesResponse,
  ListAwsSecurityGroupsRequest,
  ListAwsSecurityGroupsResponse,
  ListAwsSubnetsRequest,
  ListAwsSubnetsResponse,
  ListEksClustersRequest,
  ListEksClustersResponse,
  RdsEngineIdentifier,
  Regions,
  SecurityGroup,
  SecurityGroupRule,
  Subnet,
  UserTask,
  UserTaskDetail,
  UserTasksListResponse,
} from './types';

export const integrationService = {
  fetchExportedIntegrationCA(
    clusterId: string,
    integrationName: string
  ): Promise<ExportedIntegrationCaResponse> {
    return api.get(cfg.getIntegrationCaUrl(clusterId, integrationName));
  },

  fetchIntegration<T>(name: string): Promise<T> {
    return api
      .get(cfg.getIntegrationsUrl(name))
      .then(resp => makeIntegration(resp) as T);
  },

  fetchIntegrations(): Promise<IntegrationListResponse> {
    return api.get(cfg.getIntegrationsUrl()).then(resp => {
      const integrations = resp?.items ?? [];
      return {
        items: integrations.map(makeIntegration),
        nextKey: resp?.nextKey,
      };
    });
  },

  createIntegration<T extends IntegrationCreateRequest>(
    req: T
  ): Promise<IntegrationCreateResult<T>> {
    return api
      .post(cfg.getIntegrationsUrl(), req)
      .then(resp => makeIntegration(resp) as IntegrationCreateResult<T>);
  },

  pingAwsOidcIntegration(
    urlParams: {
      integrationName: string;
      clusterId: string;
    },
    req: AwsOidcPingRequest
  ): Promise<AwsOidcPingResponse> {
    return api.post(cfg.getPingAwsOidcIntegrationUrl(urlParams), req);
  },

  updateIntegration<T extends IntegrationUpdateRequest>(
    name: string,
    req: T
  ): Promise<IntegrationUpdateResult<T>> {
    return api
      .put(cfg.getIntegrationsUrl(name), req)
      .then(resp => makeIntegration(resp) as IntegrationUpdateResult<T>);
  },

  updateIntegrationOAuthSecret(
    name: string,
    secret: string
  ): Promise<Integration> {
    return api
      .put(cfg.getIntegrationsUrl(name), { oauth: { secret } })
      .then(makeIntegration);
  },

  deleteIntegration(req: IntegrationDeleteRequest): Promise<void> {
    return api
      .delete(cfg.getDeleteIntegrationUrlV2(req))
      .catch(err => withGenericUnsupportedError(err, 'v17.3.0'));
  },

  fetchThumbprint(): Promise<string> {
    return api.get(cfg.api.thumbprintPath);
  },

  fetchAwsRdsRequiredVpcs(
    integrationName: string,
    body: { region: string; accountId: string }
  ): Promise<Record<string, string[]>> {
    return api
      .post(cfg.getAwsRdsDbRequiredVpcsUrl(integrationName), body)
      .then(resp => resp.vpcMapOfSubnets);
  },

  fetchAwsDatabasesVpcs(
    integrationName: string,
    clusterId: string,
    body: { region: string; accountId: string; nextToken: string }
  ): Promise<AwsDatabaseVpcsResponse> {
    return api
      .post(cfg.getAwsDatabaseVpcsUrl(integrationName, clusterId), body)
      .then(resp => {
        const vpcs = resp.vpcs || [];
        return { vpcs, nextToken: resp.nextToken };
      });
  },

  /**
   * Grabs a page for rds instances and rds clusters.
   * Used with auto discovery to display "all" the
   * rds's in a region by page.
   */
  fetchAllAwsRdsEnginesDatabases(
    integrationName: string,
    req: {
      region: Regions;
      instancesNextToken?: string;
      clustersNextToken?: string;
      vpcId?: string;
    }
  ): Promise<ListAwsRdsFromAllEnginesResponse> {
    const makeResponse = response => {
      const dbs = response?.databases ?? [];
      const madeResponse: ListAwsRdsDatabaseResponse = {
        databases: dbs.map(makeAwsDatabase),
        nextToken: response?.nextToken,
      };
      return madeResponse;
    };

    return Promise.allSettled([
      api
        .post(cfg.getAwsRdsDbListUrl(integrationName), {
          region: req.region,
          vpcId: req.vpcId,
          nextToken: req.instancesNextToken,
          rdsType: 'instance',
          engines: ['mysql', 'mariadb', 'postgres'],
        })
        .then(makeResponse),
      api
        .post(cfg.getAwsRdsDbListUrl(integrationName), {
          region: req.region,
          vpcId: req.vpcId,
          nextToken: req.clustersNextToken,
          rdsType: 'cluster',
          engines: ['aurora-mysql', 'aurora-postgresql'],
        })
        .then(makeResponse),
    ]).then(response => {
      const [instances, clusters] = response;

      if (instances.status === 'rejected' && clusters.status === 'rejected') {
        // Just return one error message, likely the other will be the same error.
        throw new Error(instances.reason);
      }

      let madeResponse: ListAwsRdsFromAllEnginesResponse = {
        databases: [],
      };

      if (instances.status === 'fulfilled') {
        madeResponse = {
          databases: instances.value.databases,
          instancesNextToken: instances.value.nextToken,
        };
      } else {
        madeResponse = {
          ...madeResponse,
          oneOfError: `Failed to fetch RDS instances: ${instances.reason}`,
        };
      }

      if (clusters.status === 'fulfilled') {
        madeResponse = {
          ...madeResponse,
          databases: [...madeResponse.databases, ...clusters.value.databases],
          clustersNextToken: clusters.value.nextToken,
        };
      } else {
        madeResponse = {
          ...madeResponse,
          oneOfError: `Failed to fetch RDS clusters: ${clusters.reason}`,
        };
      }

      // Sort databases by their names
      madeResponse.databases = madeResponse.databases.sort((a, b) =>
        a.name.localeCompare(b.name)
      );

      return madeResponse;
    });
  },

  fetchAwsRdsDatabases(
    integrationName: string,
    rdsEngineIdentifier: RdsEngineIdentifier,
    req: {
      region: Regions;
      nextToken?: string;
      vpcId?: string;
    }
  ): Promise<ListAwsRdsDatabaseResponse> {
    let body: AwsOidcListDatabasesRequest;
    switch (rdsEngineIdentifier) {
      case 'mysql':
        body = {
          ...req,
          rdsType: 'instance',
          engines: ['mysql', 'mariadb'],
        };
        break;
      case 'postgres':
        body = {
          ...req,
          rdsType: 'instance',
          engines: ['postgres'],
        };
        break;
      case 'aurora-mysql':
        body = {
          ...req,
          rdsType: 'cluster',
          engines: ['aurora-mysql'],
        };
        break;
      case 'aurora-postgres':
        body = {
          ...req,
          rdsType: 'cluster',
          engines: ['aurora-postgresql'],
        };
        break;
    }

    return api
      .post(cfg.getAwsRdsDbListUrl(integrationName), body)
      .then(json => {
        const dbs = json?.databases ?? [];
        return {
          databases: dbs.map(makeAwsDatabase),
          nextToken: json?.nextToken,
        };
      });
  },

  async deployAwsOidcService(
    integrationName,
    req: AwsOidcDeployServiceRequest
  ): Promise<string> {
    const challenge = await auth.getMfaChallenge({
      scope: MfaChallengeScope.ADMIN_ACTION,
      allowReuse: true,
      isMfaRequiredRequest: {
        admin_action: {},
      },
    });

    const response = await auth.getMfaChallengeResponse(challenge);

    return api
      .post(
        cfg.getAwsDeployTeleportServiceUrl(integrationName),
        req,
        null,
        response
      )
      .then(resp => resp.serviceDashboardUrl);
  },

  async createAwsAppAccessV2(
    integrationName,
    req: CreateAwsAppAccessRequest
  ): Promise<App> {
    return (
      api
        .post(cfg.getAwsAppAccessUrlV2(integrationName), req)
        .then(makeApp)
        // TODO(kimlisa): DELETE IN 19.0
        .catch(withUnsupportedLabelFeatureErrorConversion)
    );
  },

  // TODO(kimlisa): DELETE IN 19.0
  // replaced by createAwsAppAccessV2 that accepts request body
  async createAwsAppAccess(integrationName): Promise<App> {
    return api
      .post(cfg.getAwsAppAccessUrl(integrationName), null)
      .then(makeApp);
  },

  async deployDatabaseServices(
    integrationName,
    req: AwsOidcDeployDatabaseServicesRequest
  ): Promise<string> {
    const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(true);

    return api
      .post(
        cfg.getAwsRdsDbsDeployServicesUrl(integrationName),
        req,
        null,
        mfaResponse
      )
      .then(resp => resp.clusterDashboardUrl);
  },

  async enrollEksClustersV2(
    integrationName: string,
    req: EnrollEksClustersRequest
  ): Promise<EnrollEksClustersResponse> {
    const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(true);

    return (
      api
        .post(
          cfg.getEnrollEksClusterUrlV2(integrationName),
          req,
          null,
          mfaResponse
        )
        // TODO(kimlisa): DELETE IN 19.0
        .catch(withUnsupportedLabelFeatureErrorConversion)
    );
  },

  // TODO(kimlisa): DELETE IN 19.0 - replaced by v2 endpoint.
  // replaced by enrollEksClustersV2 that accepts labels.
  async enrollEksClusters(
    integrationName: string,
    req: Omit<EnrollEksClustersRequest, 'extraLabels'>
  ): Promise<EnrollEksClustersResponse> {
    const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(true);

    return api.post(
      cfg.getEnrollEksClusterUrl(integrationName),
      req,
      null,
      mfaResponse
    );
  },

  fetchEksClusters(
    integrationName: string,
    req: ListEksClustersRequest
  ): Promise<ListEksClustersResponse> {
    return api
      .post(cfg.getListEKSClustersUrl(integrationName), req)
      .then(json => {
        const eksClusters = json?.clusters ?? [];
        return {
          clusters: eksClusters,
          nextToken: json?.nextToken,
        };
      });
  },

  // Returns a list of VPC Security Groups using the ListSecurityGroups action of the AWS OIDC Integration.
  fetchSecurityGroups(
    integrationName,
    req: ListAwsSecurityGroupsRequest
  ): Promise<ListAwsSecurityGroupsResponse> {
    return api
      .post(cfg.getListSecurityGroupsUrl(integrationName), req)
      .then(json => {
        const securityGroups = json?.securityGroups ?? [];

        return {
          securityGroups: securityGroups.map(makeSecurityGroup),
          nextToken: json?.nextToken,
        };
      });
  },

  fetchAwsSubnets(
    integrationName: string,
    clusterId: string,
    req: ListAwsSubnetsRequest
  ): Promise<ListAwsSubnetsResponse> {
    return api
      .post(cfg.getAwsSubnetListUrl(integrationName, clusterId), req)
      .then(json => {
        const subnets = json?.subnets ?? [];

        return {
          subnets: subnets.map(makeAwsSubnets),
          nextToken: json?.nextToken,
        };
      });
  },

  fetchIntegrationStats(name: string): Promise<IntegrationWithSummary> {
    return api.get(cfg.getIntegrationStatsUrl(name)).then(resp => {
      return resp;
    });
  },

  fetchIntegrationRules(
    name: string,
    resourceType: AwsResource,
    regions?: string[]
  ): Promise<IntegrationDiscoveryRules> {
    return api
      .get(cfg.getIntegrationRulesUrl(name, resourceType, regions))
      .then(resp => {
        return {
          rules: resp?.rules || [],
          nextKey: resp?.nextKey,
        };
      });
  },

  fetchAwsOidcDatabaseServices(
    name: string,
    resourceType: AwsResource,
    regions: string[]
  ): Promise<AWSOIDCListDeployedDatabaseServiceResponse> {
    return api
      .post(cfg.getAwsOidcDatabaseServices(name, resourceType, regions), null)
      .then(resp => {
        return { services: makeDatabaseServices(resp) };
      });
  },

  fetchIntegrationUserTasksList(
    name: string,
    state: TaskState
  ): Promise<UserTasksListResponse> {
    return api
      .get(cfg.getIntegrationUserTasksListUrl(name, state))
      .then(resp => {
        return {
          items: resp?.items || [],
          nextKey: resp?.nextKey,
        };
      });
  },

  fetchUserTask(name: string): Promise<UserTaskDetail> {
    return api.get(cfg.getUserTaskUrl(name)).then(resp => {
      return {
        name: resp.name,
        taskType: resp.taskType,
        state: resp.state,
        issueType: resp.issueType,
        integration: resp.integration,
        lastStateChange: resp.lastStateChange,
        description: resp.description,
        discoverEc2: {
          instances: resp.discoverEc2?.instances,
          accountId: resp.discoverEc2?.accountId,
          region: resp.discoverEc2?.region,
          ssmDocument: resp.discoverEc2?.ssmDocument,
          installerScript: resp.discoverEc2?.installerScript,
        },
        discoverEks: {
          clusters: resp.discoverEks?.instances,
          accountId: resp.discoverEks?.accountId,
          region: resp.discoverEks?.region,
          appAutoDiscover: resp.discoverEks?.appAutoDiscover,
        },
        discoverRds: {
          databases: resp.discoverRds?.instances,
          accountId: resp.discoverRds?.accountId,
          region: resp.discoverRds?.region,
        },
      };
    });
  },

  resolveUserTask(name: string): Promise<UserTask> {
    return api
      .put(cfg.getResolveUserTaskUrl(name), {
        state: TaskState.Resolved,
      })
      .then(resp => {
        return {
          name: resp.name,
          taskType: resp.taskType,
          state: resp.state,
          issueType: resp.issueType,
          integration: resp.integration,
          lastStateChange: resp.lastStateChange,
        };
      });
  },
};

function makeDatabaseServices(json: any): AWSOIDCDeployedDatabaseService[] {
  json = json ?? {};
  const { services } = json;

  return services?.map((service: AWSOIDCDeployedDatabaseService) => ({
    name: service.name ?? '',
    dashboardUrl: service.dashboardUrl ?? '',
    validTeleportConfig: service.validTeleportConfig ?? false,
    matchingLabels: service.matchingLabels ?? [],
  }));
}

export function makeIntegrations(json: any): Integration[] {
  json = json || [];
  return json.map(user => makeIntegration(user));
}

function makeIntegration(json: any): Integration {
  json = json || {};
  const { name, subKind, awsoidc, github } = json;

  const commonFields = {
    name,
    kind: subKind,
    // The integration resource does not have a "status" field, but is
    // a required field for the table that lists both plugin and
    // integration resources together. As discussed, the only
    // supported status for integration is `Running` for now:
    // https://github.com/gravitational/teleport/pull/22556#discussion_r1158674300
    statusCode: IntegrationStatusCode.Running,
  };

  if (subKind === IntegrationKind.AwsOidc) {
    return {
      ...commonFields,
      resourceType: 'integration',
      details:
        'Enroll EC2, RDS and EKS resources or enable Web/CLI access to your AWS Account.',
      spec: {
        roleArn: awsoidc?.roleArn,
        issuerS3Bucket: awsoidc?.issuerS3Bucket,
        issuerS3Prefix: awsoidc?.issuerS3Prefix,
        audience: awsoidc?.audience,
      },
    };
  }

  if (subKind === IntegrationKind.GitHub) {
    return {
      ...commonFields,
      resourceType: 'integration',
      details: `GitHub repository access for organization "${github.organization}"`,
      spec: {
        organization: github.organization,
      },
    };
  }

  return {
    ...commonFields,
    resourceType: 'integration',
  };
}

export function makeAwsDatabase(json: any): AwsRdsDatabase {
  json = json ?? {};
  const { aws, name, uri, labels, protocol } = json;

  return {
    engine: protocol,
    name,
    uri,
    status: aws?.status,
    labels: labels ?? [],
    subnets: aws?.rds?.subnets ?? [],
    resourceId: aws?.rds?.resource_id,
    vpcId: aws?.rds?.vpc_id,
    securityGroups: aws?.rds?.security_groups ?? [],
    accountId: aws?.account_id,
    region: aws?.region,
  };
}

function makeSecurityGroup(json: any): SecurityGroup {
  json = json ?? {};

  const { name, id, description = '', inboundRules, outboundRules } = json;

  return {
    name,
    id,
    description,
    inboundRules: inboundRules?.map(rule => makeSecurityGroupRule(rule)) ?? [],
    outboundRules:
      outboundRules?.map(rule => makeSecurityGroupRule(rule)) ?? [],
  };
}

function makeSecurityGroupRule(json: any): SecurityGroupRule {
  json = json ?? {};
  const { ipProtocol, fromPort, toPort, cidrs, groups } = json;

  return {
    ipProtocol,
    fromPort,
    toPort,
    cidrs: cidrs ?? [],
    groups: groups ?? [],
  };
}

function makeAwsSubnets(json: any): Subnet {
  json = json ?? {};

  const { name, id, availability_zone } = json;

  return {
    name,
    id,
    availabilityZone: availability_zone,
  };
}
