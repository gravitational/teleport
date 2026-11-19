/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { UnifiedResourceKind } from 'teleport/services/agents';
import {
  IntegrationKind,
  IntegrationWithSummary,
} from 'teleport/services/integrations';
import { Labels } from 'teleport/services/resources';

export type SetupAccessRouteState = {
  integrationName: string;
  integrationKind: IntegrationKind;
  labels?: AwsResourceLabels;
  // The resource tab to open first on the access list's define access step.
  // Defaults to applications tab if not defined.
  initialResourceKind?: AwsResourceKind;
};

export type AwsResourceLabels = Partial<
  Record<'node_labels' | 'db_labels' | 'kubernetes_labels', Labels>
>;

export type AwsResourceKind = Extract<
  UnifiedResourceKind,
  'node' | 'db' | 'kube_cluster'
>;

export type SetupAccessEnrollment = {
  ec2Enrolled: boolean;
  rdsEnrolled: boolean;
  eksEnrolled: boolean;
  azureEnrolled: boolean;
  // Undefined until a resource is enrolled.
  startAccessListWizardWith?: AwsResourceKind;
};

export function getSetupAccessEnrollment(
  stats: IntegrationWithSummary | undefined
): SetupAccessEnrollment {
  const enrollment = {
    ec2Enrolled: (stats?.awsec2?.resourcesEnrollmentSuccess ?? 0) > 0,
    rdsEnrolled: (stats?.awsrds?.resourcesEnrollmentSuccess ?? 0) > 0,
    eksEnrolled: (stats?.awseks?.resourcesEnrollmentSuccess ?? 0) > 0,
    azureEnrolled: (stats?.azurevm?.resourcesEnrollmentSuccess ?? 0) > 0,
  };

  if (stats?.subKind === IntegrationKind.AzureOidc) {
    return {
      ...enrollment,
      startAccessListWizardWith: enrollment.azureEnrolled ? 'node' : undefined,
    };
  }
  // Prioritize displaying ec2 resources b/c it has the broadest
  // filter applied, so it needs more attention than the others.
  if (enrollment.ec2Enrolled) {
    return { ...enrollment, startAccessListWizardWith: 'node' };
  }
  if (enrollment.rdsEnrolled) {
    return { ...enrollment, startAccessListWizardWith: 'db' };
  }
  if (enrollment.eksEnrolled) {
    return { ...enrollment, startAccessListWizardWith: 'kube_cluster' };
  }

  return enrollment;
}

// Returns AWS label matchers.
// Legacy labels are used to ensure we target both old and new agents.
// RFD 0361 added consistent new labeling between AWS discovered resources
// but existing agents (eks and ec2) require the user to go and manually update
// these agents config by hand typing the labels and restarting it
// (updating the agent does NOT update its labels).
export function getAwsResourceMatchingLabels({
  accountId,
  ec2Enrolled,
  rdsRegions,
  eksRegions,
}: {
  accountId: string;
  ec2Enrolled: boolean;
  rdsRegions?: string[];
  eksRegions?: string[];
}): AwsResourceLabels {
  const labels: AwsResourceLabels = {};

  if (ec2Enrolled) {
    // This is really broad and doesn't really say "aws resources"
    // but provides a better starting point for user
    // to fine tune it by selecting their own labels from here.
    labels.node_labels = { 'teleport.dev/account-id': accountId };
  }
  if (rdsRegions) {
    labels.db_labels = getLegacyAwsLabels(accountId, rdsRegions);
  }
  if (eksRegions) {
    labels.kubernetes_labels = getLegacyAwsLabels(accountId, eksRegions);
  }

  return labels;
}

function getLegacyAwsLabels(accountId: string, regions: string[]): Labels {
  const labels: Labels = {
    'account-id': accountId,
    'teleport.dev/cloud': 'AWS',
  };
  // Not including regions is same thing as allowing all regions (wildcard).
  if (regions.length > 0 && !regions.includes('*')) {
    labels.region = regions;
  }
  return labels;
}
