/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';
import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import { UnifiedResourceKind } from 'teleport/services/agents';
import {
  AzureResource,
  IntegrationDiscoveryRule,
} from 'teleport/services/integrations';

type CloudFilter =
  | {
      resource: AwsResource;
      integrationName: string;
      isOrganization: boolean;
      accountId: string;
    }
  | { resource: AzureResource };

type BaseParams = {
  rules: IntegrationDiscoveryRule[];
};

export type ListResourceRouteParams = BaseParams & CloudFilter;

export function getListResourceRoute(params: ListResourceRouteParams): string {
  let predicates: string[] = [];
  let resourceKind: UnifiedResourceKind;
  const { resource } = params;

  // Organizations have to be best attempt filtering, since it targets multi accounts
  // and wildcard may be used.
  switch (resource) {
    case AzureResource.vm:
      resourceKind = 'node';
      predicates = getAzureListResourcePredicate(params.rules);
      break;

    case AwsResource.ec2:
      resourceKind = 'node';
      predicates.push('exists(labels["teleport.dev/instance-id"])');
      if (params.isOrganization) {
        predicates.push('exists(labels["teleport.dev/account-id"])');
      } else {
        // TODO(kimlisa): by v21, add the new `teleport.dev/aws-region`
        // for (params.rule.region) to fine tune filtering.
        predicates.push(
          `labels["teleport.dev/account-id"] == "${params.accountId}"`
        );
      }
      break;

    case AwsResource.eks:
      resourceKind = 'kube_cluster';
      predicates.push('labels["teleport.dev/cloud"] == "AWS"');
      // TODO(kimlisa): by v21, replace with new `teleport.dev/account-id`
      const labelAccountId = 'account-id';
      if (params.isOrganization) {
        predicates.push(`exists(labels["${labelAccountId}"])`);
      } else {
        predicates.push(`labels["${labelAccountId}"] == "${params.accountId}"`);
        const regions = params.rules.map(rule => rule.region);
        if (regions.length) {
          // TODO(kimlisa): by v21, replace with new `teleport.dev/aws-region`
          predicates.push(buildLabelFilter(`region`, [...new Set(regions)]));
        }
      }
      break;

    case AwsResource.rds:
      resourceKind = 'db';
      predicates.push(
        `labels["teleport.internal/discovery-integration-name"] == "${params.integrationName}"`
      );
      break;

    default:
      resource satisfies never;
      return '';
  }

  return encodeUrlQueryParams({
    pathname: cfg.getUnifiedResourcesRoute(cfg.proxyCluster),
    searchString: predicates.join(' && '),
    isAdvancedSearch: true,
    kinds: [resourceKind],
  });
}

function getAzureListResourcePredicate(rules: IntegrationDiscoveryRule[]) {
  const predicates: string[] = [];
  // TODO(kimlisa): DELETE IN v21 (see other TODO in this file)
  // Replace the legacy `teleport.internal` with `teleport.dev`. v19+ agents set
  // both the "teleport.internal" (for backwards compat) and the newer "teleport.dev".
  const labelPrefix = 'teleport.internal/';

  // Best attempt azure resource filtering.
  // There is no label that identifies it as "azure", but a combination
  // of matching labels can indicate azure resource.
  const subs = rules.flatMap(rule => rule.subscriptions ?? []);
  if (subs.length) {
    predicates.push(
      buildLabelFilter(`${labelPrefix}subscription-id`, [...new Set(subs)])
    );
  }

  const regions = rules.map(rule => rule.region);
  if (regions.length) {
    predicates.push(
      buildLabelFilter(`${labelPrefix}region`, [...new Set(regions)])
    );
  }

  const rgs = rules.flatMap(rule => rule.resourceGroups ?? []);
  if (rgs.length) {
    predicates.push(
      buildLabelFilter(`${labelPrefix}resource-group`, [...new Set(rgs)])
    );
  }
  predicates.push(`exists(labels["${labelPrefix}vm-id"])`);
  return predicates;
}

function buildLabelFilter(label: string, configuredValues: string[]) {
  const values = configuredValues.filter(value => value !== '*');

  // An asterisk exists.
  if (values.length === 0 || values.length !== configuredValues.length) {
    return `exists(labels["${label}"])`;
  }

  const predicates = values.map(value => `labels["${label}"] == "${value}"`);
  return `(${predicates.join(' || ')})`;
}

export function getResourceKindFromResource(
  resourceType: AwsResource | AzureResource
) {
  switch (resourceType) {
    case AwsResource.ec2:
      return 'EC2';
    case AwsResource.eks:
      return 'EKS';
    case AwsResource.rds:
      return 'RDS';
    case AzureResource.vm:
      return 'Azure VM';
    default:
      resourceType satisfies never;
  }
}

// Used to determine if discovery rules needs to be
// fetched (returns true) for the resource type.
export function requiresDiscoveryRules(
  resource: AwsResource | AzureResource,
  isOrganization: boolean
): boolean {
  switch (resource) {
    case AzureResource.vm:
      return true;
    case AwsResource.eks:
      return !isOrganization;
    case AwsResource.ec2:
    case AwsResource.rds:
      return false;
    default:
      resource satisfies never;
      return true;
  }
}
