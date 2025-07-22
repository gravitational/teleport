/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import type { JSX } from 'react';
import { Link as InternalLink } from 'react-router-dom';

import { Mark } from 'design';
import {
  InfoExternalTextLink,
  InfoGuideConfig,
  InfoParagraph,
  InfoTitle,
  InfoUl,
  ReferenceLink,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'teleport/config';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { SelectResourceSpec } from '../SelectResource/resources';

const requiredAwsAccount =
  'AWS account with permissions to create and attach IAM policies.';

/**
 * Returns info guide content to be rendered inside the InfoGuideSidePanel
 * component.
 *
 * Content will depend on the "resourceSpec" id.
 */
export function getOverview({
  resourceSpec,
}: {
  resourceSpec: SelectResourceSpec;
}): JSX.Element | null {
  if (!resourceSpec.id) {
    return null;
  }

  let overview: JSX.Element;
  let prereqs: JSX.Element;
  let links: Record<string, ReferenceLink>;

  switch (resourceSpec.id) {
    case DiscoverGuideId.Kubernetes:
      overview = (
        <InfoParagraph>
          This guide uses Helm to install the Teleport agent into a cluster, and
          by default turns on auto-discovery of all apps in the cluster.
        </InfoParagraph>
      );
      prereqs = (
        <InfoUl>
          <li>Network egress from your Kubernetes cluster to Teleport.</li>
          <li>Helm installed on your local machine.</li>
          <li>Kubernetes API access to install the Helm chart.</li>
        </InfoUl>
      );
      break;

    case DiscoverGuideId.DatabasePostgres:
    case DiscoverGuideId.DatabaseMysql:
      overview = (
        <InfoParagraph>
          This guide configures mTLS between your Teleport proxy and your target
          database.
        </InfoParagraph>
      );
      prereqs = (
        <InfoUl>
          <li>
            Copy of the database CA certificate if it is signed by a third-party
            or private CA.
          </li>
          <li>
            Ability to modify the <Mark>pg_auth</Mark> file on the database
            server.
          </li>
          <li>
            SSH or tsh access to the server running the database, and ability to
            either SCP files, or run a command to retrieve TLS certificates from
            the Teleport cluster.
          </li>
        </InfoUl>
      );
      break;

    case DiscoverGuideId.ServerLinuxAmazon: // TODO (kimlisa) collapse linux and on page info provide supported
    case DiscoverGuideId.ServerLinuxDebian:
    case DiscoverGuideId.ServerLinuxUbuntu:
    case DiscoverGuideId.ServerLinuxRhelCentos:
    case DiscoverGuideId.ServerMac:
      overview = (
        <InfoParagraph>
          This guide sets up a single server in your Teleport cluster for SSH
          access. It uses a short-lived, randomly generated{' '}
          <InternalLink to={cfg.routes.joinTokens} target="_blank">
            join token
          </InternalLink>{' '}
          with Node permissions. It is good for getting quickly started with
          server access.
        </InfoParagraph>
      );
      prereqs = (
        <InfoUl>
          <li>SSH access to the server.</li>
          <li>Root or sudo privileges to run the script.</li>
          <li>List of OS users you want to be able to connect as.</li>
        </InfoUl>
      );
      break;

    case DiscoverGuideId.KubernetesAwsEks:
      links = {
        accessEntries: {
          title: 'Using access entries',
          href: 'https://docs.aws.amazon.com/eks/latest/userguide/setting-up-access-entries.html',
        },
      };
      overview = (
        <InfoParagraph>
          This guide is used to set up auto-enrollment of one or more EKS
          clusters with a script that you will run in AWS CloudShell.
        </InfoParagraph>
      );
      prereqs = (
        <InfoUl>
          <li>{requiredAwsAccount}</li>
          <li>
            <InfoExternalTextLink href={links.accessEntries.href}>
              Access entries
            </InfoExternalTextLink>{' '}
            authentication mode enabled in your target EKS clusters.
          </li>
          <li>
            List of Kubernetes users and groups you want Teleport users to be
            able to authenticate as.
          </li>
        </InfoUl>
      );
      break;

    case DiscoverGuideId.ApplicationAwsCliConsole:
      overview = (
        <InfoParagraph>
          This guide configures your AWS OIDC policy to allow console access
          with a script that you will run in AWS CloudShell.
        </InfoParagraph>
      );
      prereqs = (
        <InfoUl>
          <li>{requiredAwsAccount}</li>
          <li>List of AWS roles you want to be able to authenticate as.</li>
        </InfoUl>
      );
      break;

    case DiscoverGuideId.ServerAwsEc2Ssm:
      links = {
        systemsFleetManager: {
          title: 'AWS Systems/Fleet Manager',
          href: 'https://docs.aws.amazon.com/systems-manager/latest/userguide/fleet-manager.html',
        },
        ssmAgent: {
          title: 'SSM Agent',
          href: 'https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent-status-and-restart.html',
        },
        amazonSSMManagedInstanceCore: {
          title: 'AmazonSSMManagedInstanceCore policy',
          href: 'https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonSSMManagedInstanceCore.html',
        },
      };
      overview = (
        <InfoParagraph>
          This guide is used to enroll <i>all</i> EC2 instances controlled by{' '}
          <InfoExternalTextLink href={links.systemsFleetManager.href}>
            AWS Systems/Fleet Manager
          </InfoExternalTextLink>{' '}
          in a region.
        </InfoParagraph>
      );
      prereqs = (
        <InfoUl>
          <li>{requiredAwsAccount}</li>
          <li>
            List of OS users you want to be able to connect as (i.e. root,
            ubuntu, etc).
          </li>
          <li>
            <InfoExternalTextLink href={links.ssmAgent.href}>
              SSM Agent
            </InfoExternalTextLink>{' '}
            running in target EC2 instances, and have the{' '}
            <InfoExternalTextLink
              href={links.amazonSSMManagedInstanceCore.href}
            >
              AmazonSSMManagedInstanceCore
            </InfoExternalTextLink>{' '}
            policy attached to their IAM profile.
          </li>
        </InfoUl>
      );
      break;

    case DiscoverGuideId.DatabaseAwsRdsAuroraPostgres:
    case DiscoverGuideId.DatabaseAwsRdsAuroraMysql:
    case DiscoverGuideId.DatabaseAwsRdsPostgres:
    case DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb:
      links = {
        iamAuthn: {
          title: 'Creating database IAM users',
          href: getRdsIamAuthnHref(resourceSpec.id),
        },
        amazonEcs: {
          title: 'What is Amazon ECS',
          href: 'https://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html',
        },
        howEnrollmentWorks: {
          title: 'How AWS OIDC RDS enrollment work',
          href: 'https://goteleport.com/docs/admin-guides/management/guides/awsoidc-integration-rds/',
        },
      };

      overview = (
        <InfoParagraph>
          This{' '}
          <InfoExternalTextLink href={links.howEnrollmentWorks.href}>
            guide
          </InfoExternalTextLink>{' '}
          is used to set up enrollment of one or more RDS databases in a region.
          A Database Service will be deployed in{' '}
          <InfoExternalTextLink href={links.amazonEcs.href}>
            Amazon ECS
          </InfoExternalTextLink>{' '}
          that proxies to the databases.
        </InfoParagraph>
      );

      prereqs = (
        <InfoUl>
          <li>{requiredAwsAccount}</li>
          <li>
            Ability to create database{' '}
            <Mark>
              <b>
                <InfoExternalTextLink href={links.iamAuthn.href}>
                  IAM users
                </InfoExternalTextLink>
              </b>
            </Mark>{' '}
            and connect to target databases.
          </li>
          <li>The VPC of the RDS databases you want to enroll.</li>
          <li>
            At least one subnet in the VPC with a route to an internet gateway.
          </li>
          <li>
            Security groups that permit access to your RDS databases and allow
            unrestricted outbound internet traffic.
          </li>
        </InfoUl>
      );
      break;
    default:
      return null;
  }

  return (
    <>
      {overview && <>{overview}</>}
      <InfoTitle>Prerequisites</InfoTitle>
      {prereqs}
      {links && <ReferenceLinks links={Object.values(links)} />}
    </>
  );
}

export type AwsRdsGuideIds =
  | DiscoverGuideId.DatabaseAwsRdsAuroraMysql
  | DiscoverGuideId.DatabaseAwsRdsAuroraPostgres
  | DiscoverGuideId.DatabaseAwsRdsPostgres
  | DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb;

export const getRdsIamAuthnHref = (id: AwsRdsGuideIds) => {
  if (id === DiscoverGuideId.DatabaseAwsRdsAuroraMysql) {
    return 'https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html#UsingWithRDS.IAMDBAuth.DBAccounts.MySQL';
  }
  if (id === DiscoverGuideId.DatabaseAwsRdsAuroraPostgres) {
    return 'https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html#UsingWithRDS.IAMDBAuth.DBAccounts.PostgreSQL';
  }
  if (id === DiscoverGuideId.DatabaseAwsRdsPostgres) {
    return 'https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html#UsingWithRDS.IAMDBAuth.DBAccounts.PostgreSQL';
  }
  if (id === DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb) {
    return 'https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html#UsingWithRDS.IAMDBAuth.DBAccounts.MySQL';
  }
};

export function getDiscoverInfoGuideConfig(
  guide: JSX.Element
): InfoGuideConfig {
  if (!guide) {
    return;
  }
  return {
    guide: guide,
    title: 'Overview',
  };
}
