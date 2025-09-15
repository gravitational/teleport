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

import { useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Label, Link, Mark, Text } from 'design';
import { CollapsibleInfoSection } from 'design/CollapsibleInfoSection';
import { ChevronDown } from 'design/Icon';
import { SpaceProps } from 'design/system';
import { H3 } from 'design/Text';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import { Tabs } from 'teleport/components/Tabs';
import {
  AwsRdsGuideIds,
  getRdsIamAuthnHref,
} from 'teleport/Discover/Overview/Overview';
import { DatabaseServiceDeploy } from 'teleport/Discover/useDiscover';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

export function AwsRdsAuthRequirements({
  wantAutoDiscover,
  id,
  serviceDeploy,
  uri,
  ...spaceProps
}: {
  wantAutoDiscover: boolean;
  id: DiscoverGuideId;
  uri: string | undefined;
  serviceDeploy: DatabaseServiceDeploy | undefined;
} & SpaceProps) {
  const [showEnableInfo, setShowEnableInfo] = useState(false);
  const [showCreateInfo, setShowCreateInfo] = useState(false);

  if (!isAwsRds(id)) {
    return;
  }

  const isPostgres =
    id === DiscoverGuideId.DatabaseAwsRdsPostgres ||
    id === DiscoverGuideId.DatabaseAwsRdsAuroraPostgres;

  let createInfo = isPostgres ? (
    <AwsPostgresUserInfo id={id} />
  ) : (
    <AwsMysqlUserInfo id={id} />
  );
  if (wantAutoDiscover) {
    // If auto discovery was enabled, users need to see all supported engine info
    // to help setup access to their databases.
    // The id is specifically Aurora here since the AWS documentation for creating
    // IAM users are equivalent (and Teleport doc does the same).
    createInfo = (
      <Box width="100%">
        <Tabs
          tabs={[
            {
              title: 'PostgreSQL',
              content: (
                <AwsPostgresUserInfo
                  id={DiscoverGuideId.DatabaseAwsRdsAuroraPostgres}
                />
              ),
            },
            {
              title: `MySQL`,
              content: (
                <AwsMysqlUserInfo
                  id={DiscoverGuideId.DatabaseAwsRdsAuroraMysql}
                />
              ),
            },
          ]}
        />
      </Box>
    );
  }

  return (
    <Box {...spaceProps}>
      <Text mb={3}>
        Teleport uses AWS IAM authentication to connect to RDS databases.
      </Text>
      <Flex
        alignItems="center"
        onClick={() => setShowEnableInfo(!showEnableInfo)}
        gap={1}
        mb={2}
      >
        <Flex css={{ cursor: 'pointer' }}>
          <ExpandIcon expanded={showEnableInfo} />
          <Text bold>
            You must enable IAM authentication on your RDS databases
          </Text>
        </Flex>
      </Flex>
      {showEnableInfo && (
        <Box ml={4} mb={3}>
          Follow AWS{' '}
          <Link
            target="_blank"
            href="https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.Enabling.html"
          >
            documentation
          </Link>{' '}
          on how to enable IAM authentication on your{' '}
          {wantAutoDiscover ? 'databases' : 'database'}.
        </Box>
      )}

      <Flex
        alignItems="center"
        onClick={() => setShowCreateInfo(!showCreateInfo)}
        gap={1}
        mb={2}
      >
        <Flex css={{ cursor: 'pointer' }}>
          <ExpandIcon expanded={showCreateInfo} />
          <Text bold>
            You must create or alter database users to allow them to log in with
            IAM authentication
          </Text>
        </Flex>
      </Flex>
      {showCreateInfo && (
        <Box mx={4}>
          <Box width="100%">{createInfo}</Box>
          <H3 mt={4}>Connecting to your RDS Databases</H3>
          <Text mb={2}>
            AWS documents how to{' '}
            <Link target="_blank" href={getAwsRdsConnectLink(id)}>
              connect to your RDS database
            </Link>{' '}
            in various ways.
          </Text>
          {!wantAutoDiscover && uri && (
            <Box mt={2} mb={3}>
              Database URI:
              <TextSelectCopyMulti lines={[{ text: uri }]} />
            </Box>
          )}
          <CollapsibleInfoSection
            mt={3}
            size="small"
            openLabel="Connect with AWS CloudShell"
            closeLabel="Connect with AWS CloudShell"
          >
            Alternatively, you can use{' '}
            <Link
              href="https://console.aws.amazon.com/cloudshell/home"
              target="_blank"
            >
              AWS CloudShell
            </Link>{' '}
            to connect to your RDS database. See{' '}
            <Link
              target="_blank"
              href="https://docs.aws.amazon.com/cloudshell/latest/userguide/creating-vpc-environment.html"
            >
              Creating a CloudShell VPC environment
            </Link>{' '}
            for more information.
            {serviceDeploy && (
              <>
                {
                  ' Use the same security groups and subnets you specified for your database service.'
                }
                <DatabaseInfo serviceDeploy={serviceDeploy} />
              </>
            )}
          </CollapsibleInfoSection>
        </Box>
      )}
    </Box>
  );
}

function DatabaseInfo({
  serviceDeploy,
}: {
  serviceDeploy?: DatabaseServiceDeploy;
}) {
  if (serviceDeploy?.method !== 'auto') {
    return null;
  }

  let securityGroups;
  if (serviceDeploy?.selectedSecurityGroups?.length) {
    securityGroups = (
      <Box mb={2}>
        <b>Selected Security Groups:</b>{' '}
        {serviceDeploy.selectedSecurityGroups.map(sg => (
          <Label key={sg} kind="secondary" mr={2}>
            {sg}
          </Label>
        ))}
      </Box>
    );
  }

  let subnetIds;
  if (serviceDeploy?.selectedSubnetIds?.length) {
    subnetIds = (
      <Box>
        <b>Selected Subnet IDs:</b>{' '}
        {serviceDeploy.selectedSubnetIds.map(sg => (
          <Label key={sg} kind="secondary" mr={2}>
            {sg}
          </Label>
        ))}
      </Box>
    );
  }

  return (
    <Box mt={3}>
      {securityGroups}
      {subnetIds}
    </Box>
  );
}

function AwsPostgresUserInfo({ id }: { id: AwsRdsGuideIds }) {
  return (
    <Box>
      <Text mb={2}>
        Database users must have an <Mark>rds_iam</Mark> role:
      </Text>
      <TextSelectCopyMulti
        bash={false}
        lines={[
          {
            text:
              `CREATE USER YOUR_USERNAME;\n` +
              `GRANT rds_iam TO YOUR_USERNAME;`,
          },
        ]}
      />
      <CreateRdsIamAccountText id={id} />
    </Box>
  );
}

function AwsMysqlUserInfo({ id }: { id: AwsRdsGuideIds }) {
  return (
    <Box>
      <Box mb={2}>
        <Text mb={2}>
          Database users must have the RDS authentication plugin enabled:
        </Text>
        <TextSelectCopyMulti
          bash={false}
          lines={[
            {
              text: "CREATE USER alice IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS';",
            },
          ]}
        />
      </Box>
      <Box>
        <Text mb={2}>
          Created user may not have access to anything by default. You can grant
          some permissions by:
        </Text>
        <TextSelectCopyMulti
          bash={false}
          lines={[
            {
              text: "GRANT ALL ON `%`.* TO 'alice'@'%';",
            },
          ]}
        />
      </Box>
      <CreateRdsIamAccountText id={id} />
    </Box>
  );
}

function CreateRdsIamAccountText({ id }: { id: AwsRdsGuideIds }) {
  return (
    <Text mt={2}>
      See{' '}
      <Link target="_blank" href={getRdsIamAuthnHref(id)}>
        Creating a database account using IAM authentication
      </Link>{' '}
      for more information.
    </Text>
  );
}

function getAwsRdsConnectLink(id: AwsRdsGuideIds) {
  switch (id) {
    case DiscoverGuideId.DatabaseAwsRdsPostgres:
      return 'https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ConnectToPostgreSQLInstance.html';
    case DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb:
      return 'https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_ConnectToInstance.html';
    case DiscoverGuideId.DatabaseAwsRdsAuroraMysql:
      return 'https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Connecting.html#Aurora.Connecting.AuroraMySQL';
    case DiscoverGuideId.DatabaseAwsRdsAuroraPostgres:
      return 'https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Connecting.html#Aurora.Connecting.AuroraPostgreSQL';
  }
}

const ExpandIcon = styled(ChevronDown).attrs({ size: 'small' })<{
  expanded: boolean;
}>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'none' : 'rotate(-90deg)')};
  color: ${p => p.theme.colors.text.muted};
  margin-right: ${p => p.theme.space[2]}px;
`;

export function AwsRdsAuthRequirementAlert({
  wantAutoDiscover,
  id,
  uri,
  ...spaceProps
}: {
  wantAutoDiscover: boolean;
  id: DiscoverGuideId;
  uri?: string;
} & SpaceProps) {
  return (
    <Alert kind="neutral" {...spaceProps}>
      <div
        css={`
          font-weight: normal;
        `}
      >
        Your RDS databases must have password and IAM authentication enabled.
        You can set this up now, or later in the <b>Set Up Access</b> step.
        <CollapsibleInfoSection
          size="small"
          mt={3}
          openLabel="IAM Database Authentication Requirements"
          closeLabel="IAM Database Authentication Requirements"
        >
          <AwsRdsAuthRequirements
            wantAutoDiscover={wantAutoDiscover}
            id={id}
            uri={uri}
            serviceDeploy={undefined}
          />
        </CollapsibleInfoSection>
      </div>
    </Alert>
  );
}

export function isAwsRds(id: DiscoverGuideId) {
  return (
    id === DiscoverGuideId.DatabaseAwsRdsAuroraMysql ||
    id === DiscoverGuideId.DatabaseAwsRdsAuroraPostgres ||
    id === DiscoverGuideId.DatabaseAwsRdsPostgres ||
    id === DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb
  );
}
