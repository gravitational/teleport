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

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import makeNode from '../nodes/makeNode';
import auth from '../auth/auth';
import { App } from '../apps';
import makeApp from '../apps/makeApps';

import {
  Integration,
  IntegrationCreateRequest,
  IntegrationUpdateRequest,
  IntegrationStatusCode,
  IntegrationListResponse,
  AwsOidcListDatabasesRequest,
  AwsRdsDatabase,
  ListAwsRdsDatabaseResponse,
  RdsEngineIdentifier,
  AwsOidcDeployServiceRequest,
  ListEc2InstancesRequest,
  ListEc2InstancesResponse,
  Ec2InstanceConnectEndpoint,
  ListEc2InstanceConnectEndpointsRequest,
  ListEc2InstanceConnectEndpointsResponse,
  ListAwsSecurityGroupsRequest,
  ListAwsSecurityGroupsResponse,
  DeployEc2InstanceConnectEndpointRequest,
  DeployEc2InstanceConnectEndpointResponse,
  SecurityGroup,
  ListEksClustersResponse,
  EnrollEksClustersResponse,
  EnrollEksClustersRequest,
  ListEksClustersRequest,
  AwsOidcDeployDatabaseServicesRequest,
  Regions,
  ListAwsRdsFromAllEnginesResponse,
  ListAwsSubnetsRequest,
  ListAwsSubnetsResponse,
  Subnet,
  AwsDatabaseVpcsResponse,
} from './types';

export const integrationService = {
  fetchIntegration(name: string): Promise<Integration> {
    return api.get(cfg.getIntegrationsUrl(name)).then(makeIntegration);
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

  createIntegration(req: IntegrationCreateRequest): Promise<Integration> {
    return api.post(cfg.getIntegrationsUrl(), req).then(makeIntegration);
  },

  updateIntegration(
    name: string,
    req: IntegrationUpdateRequest
  ): Promise<Integration> {
    return api.put(cfg.getIntegrationsUrl(name), req).then(makeIntegration);
  },

  deleteIntegration(name: string): Promise<void> {
    return api.delete(cfg.getIntegrationsUrl(name));
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
    const webauthnResponse = await auth.getWebauthnResponseForAdminAction(true);

    return api
      .post(
        cfg.getAwsDeployTeleportServiceUrl(integrationName),
        req,
        null,
        webauthnResponse
      )
      .then(resp => resp.serviceDashboardUrl);
  },

  async createAwsAppAccess(integrationName): Promise<App> {
    return api
      .post(cfg.getAwsAppAccessUrl(integrationName), null)
      .then(makeApp);
  },

  async deployDatabaseServices(
    integrationName,
    req: AwsOidcDeployDatabaseServicesRequest
  ): Promise<string> {
    const webauthnResponse = await auth.getWebauthnResponseForAdminAction(true);

    return api
      .post(
        cfg.getAwsRdsDbsDeployServicesUrl(integrationName),
        req,
        null,
        webauthnResponse
      )
      .then(resp => resp.clusterDashboardUrl);
  },

  async enrollEksClusters(
    integrationName: string,
    req: EnrollEksClustersRequest
  ): Promise<EnrollEksClustersResponse> {
    const webauthnResponse = await auth.getWebauthnResponseForAdminAction(true);

    return api.post(
      cfg.getEnrollEksClusterUrl(integrationName),
      req,
      null,
      webauthnResponse
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

  // Returns a list of EC2 Instances using the ListEC2ICE action of the AWS OIDC Integration.
  fetchAwsEc2Instances(
    integrationName,
    req: ListEc2InstancesRequest
  ): Promise<ListEc2InstancesResponse> {
    return api
      .post(cfg.getListEc2InstancesUrl(integrationName), req)
      .then(json => {
        const instances = json?.servers ?? [];
        return {
          instances: instances.map(makeNode),
          nextToken: json?.nextToken,
        };
      });
  },

  // Returns a list of EC2 Instance Connect Endpoints using the ListEC2ICE action of the AWS OIDC Integration.
  fetchAwsEc2InstanceConnectEndpoints(
    integrationName,
    req: ListEc2InstanceConnectEndpointsRequest
  ): Promise<ListEc2InstanceConnectEndpointsResponse> {
    return api
      .post(cfg.getListEc2InstanceConnectEndpointsUrl(integrationName), req)
      .then(json => {
        const endpoints = json?.ec2Ices ?? [];

        return {
          endpoints: endpoints.map(makeEc2InstanceConnectEndpoint),
          nextToken: json?.nextToken,
          dashboardLink: json?.dashboardLink,
        };
      });
  },

  // Deploys an EC2 Instance Connect Endpoint.
  deployAwsEc2InstanceConnectEndpoints(
    integrationName,
    req: DeployEc2InstanceConnectEndpointRequest
  ): Promise<DeployEc2InstanceConnectEndpointResponse> {
    return api
      .post(cfg.getDeployEc2InstanceConnectEndpointUrl(integrationName), req)
      .then(resp => {
        return resp ?? [];
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
};

export function makeIntegrations(json: any): Integration[] {
  json = json || [];
  return json.map(user => makeIntegration(user));
}

function makeIntegration(json: any): Integration {
  json = json || {};
  const { name, subKind, awsoidc } = json;
  return {
    resourceType: 'integration',
    name,
    kind: subKind,
    spec: {
      roleArn: awsoidc?.roleArn,
      issuerS3Bucket: awsoidc?.issuerS3Bucket,
      issuerS3Prefix: awsoidc?.issuerS3Prefix,
    },
    // The integration resource does not have a "status" field, but is
    // a required field for the table that lists both plugin and
    // integration resources together. As discussed, the only
    // supported status for integration is `Running` for now:
    // https://github.com/gravitational/teleport/pull/22556#discussion_r1158674300
    statusCode: IntegrationStatusCode.Running,
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
    subnets: aws?.rds?.subnets,
    resourceId: aws?.rds?.resource_id,
    vpcId: aws?.rds?.vpc_id,
    accountId: aws?.account_id,
    region: aws?.region,
  };
}

function makeEc2InstanceConnectEndpoint(json: any): Ec2InstanceConnectEndpoint {
  json = json ?? {};
  const { name, state, stateMessage, dashboardLink, subnetId, vpcId } = json;

  return {
    name,
    state,
    stateMessage,
    dashboardLink,
    subnetId,
    vpcId,
  };
}

function makeSecurityGroup(json: any): SecurityGroup {
  json = json ?? {};

  const { name, id, description = '', inboundRules, outboundRules } = json;

  return {
    name,
    id,
    description,
    inboundRules: inboundRules ?? [],
    outboundRules: outboundRules ?? [],
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
