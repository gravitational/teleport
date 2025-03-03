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

import React, { useEffect, useState } from 'react';

import { Box, Flex, H3, Link, Mark, Text } from 'design';
import { Info as InfoIcon } from 'design/Icon';

import { Tabs } from 'teleport/components/Tabs';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { StyledBox } from 'teleport/Discover/Shared';
import {
  Option,
  SelectCreatable,
} from 'teleport/Discover/Shared/SelectCreatable';
import {
  SetupAccessWrapper,
  useUserTraits,
  type State,
} from 'teleport/Discover/Shared/SetupAccess';
import { DbMeta } from 'teleport/Discover/useDiscover';

import { DatabaseEngine, DatabaseLocation } from '../../SelectResource';

export default function Container() {
  const state = useUserTraits();
  return <SetupAccess {...state} />;
}

export function SetupAccess(props: State) {
  const {
    onProceed,
    initSelectedOptions,
    getFixedOptions,
    getSelectableOptions,
    resourceSpec,
    onPrev,
    agentMeta,
    ...restOfProps
  } = props;
  const [nameInputValue, setNameInputValue] = useState('');
  const [selectedNames, setSelectedNames] = useState<Option[]>([]);

  const [userInputValue, setUserInputValue] = useState('');
  const [selectedUsers, setSelectedUsers] = useState<Option[]>([]);

  const wantAutoDiscover = !!agentMeta.autoDiscovery;

  useEffect(() => {
    if (props.attempt.status === 'success') {
      setSelectedNames(initSelectedOptions('databaseNames'));
      setSelectedUsers(initSelectedOptions('databaseUsers'));
    }
  }, [props.attempt.status, initSelectedOptions]);

  function handleNameKeyDown(event: React.KeyboardEvent) {
    if (!nameInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setSelectedNames([
          ...selectedNames,
          { value: nameInputValue, label: nameInputValue },
        ]);
        setNameInputValue('');
        event.preventDefault();
    }
  }

  function handleUserKeyDown(event: React.KeyboardEvent) {
    if (!userInputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
        setSelectedUsers([
          ...selectedUsers,
          { value: userInputValue, label: userInputValue },
        ]);
        setUserInputValue('');
        event.preventDefault();
    }
  }

  function handleOnProceed() {
    let numStepsToIncrement;
    // Skip test connection since test connection currently
    // only supports one resource testing and auto enrolling
    // enrolls resources > 1.
    if (wantAutoDiscover) {
      numStepsToIncrement = 2;
    }
    onProceed(
      { databaseNames: selectedNames, databaseUsers: selectedUsers },
      numStepsToIncrement
    );
  }

  const { engine, location } = resourceSpec.dbMeta;
  let hasTraits = selectedUsers.length > 0;
  // Postgres connection testing requires both db user and a db name.
  if (engine === DatabaseEngine.Postgres) {
    hasTraits = hasTraits && selectedNames.length > 0;
  }

  const canAddTraits = !props.isSsoUser && props.canEditUser;
  const headerSubtitle =
    'Allow access from your Database names and users to interact with your Database.';

  const dbMeta = agentMeta as DbMeta;

  let infoContent = (
    <StyledBox mt={5}>
      <Info dbEngine={engine} dbLocation={location} />
    </StyledBox>
  );
  if (wantAutoDiscover) {
    infoContent = <AutoDiscoverInfoTabs location={location} />;
  }

  return (
    <SetupAccessWrapper
      {...restOfProps}
      headerSubtitle={headerSubtitle}
      traitKind="Database"
      traitDescription="names and users"
      hasTraits={hasTraits}
      onProceed={handleOnProceed}
      infoContent={infoContent}
      // Don't allow going back to previous screen when deploy db
      // service got skipped or user auto deployed the db service.
      onPrev={dbMeta.serviceDeployedMethod === 'manual' ? onPrev : null}
      wantAutoDiscover={wantAutoDiscover}
    >
      {wantAutoDiscover && (
        <Text mb={3}>
          Since auto-discovery is enabled, make sure to include all database
          users and names that will be used to connect to the discovered
          databases.
        </Text>
      )}
      <Box mb={4}>
        Database Users
        <SelectCreatable
          inputValue={userInputValue}
          isClearable={selectedUsers.some(v => !v.isFixed)}
          onInputChange={setUserInputValue}
          onKeyDown={handleUserKeyDown}
          placeholder="Start typing database users and press enter"
          value={selectedUsers}
          isDisabled={!canAddTraits}
          onChange={(value, action) => {
            if (action.action === 'clear') {
              setSelectedUsers(getFixedOptions('databaseUsers'));
            } else {
              setSelectedUsers(value || []);
            }
          }}
          options={getSelectableOptions('databaseUsers')}
          autoFocus
        />
      </Box>
      <Box mb={2}>
        Database Names
        <SelectCreatable
          inputValue={nameInputValue}
          isClearable={selectedNames.some(v => !v.isFixed)}
          onInputChange={setNameInputValue}
          onKeyDown={handleNameKeyDown}
          placeholder="Start typing database names and press enter"
          value={selectedNames}
          isDisabled={!canAddTraits}
          onChange={(value, action) => {
            if (action.action === 'clear') {
              setSelectedNames(getFixedOptions('databaseNames'));
            } else {
              setSelectedNames(value || []);
            }
          }}
          options={getSelectableOptions('databaseNames')}
        />
      </Box>
    </SetupAccessWrapper>
  );
}

const Info = (props: {
  dbEngine: DatabaseEngine;
  dbLocation: DatabaseLocation;
}) => (
  <>
    <Flex mb={2}>
      <InfoIcon size="medium" mr={1} />
      <H3>To allow access using your Database Users</H3>
    </Flex>
    <DbEngineInstructions {...props} />
    <Box>
      <H3>Access Definition</H3>
      <ul
        css={`
          margin-bottom: 0;
        `}
      >
        <li>
          <Mark>Database User</Mark> is the name of a user that is allowed to
          connect to a database. A wildcard allows any user.
        </li>
        <li>
          <Mark>Database Name</Mark> is the name of a logical database (aka
          schemas) that a <Mark>Database User</Mark> will be allowed to connect
          to within a database server. A wildcard allows any database.
        </li>
      </ul>
    </Box>
  </>
);

function DbEngineInstructions({
  dbEngine,
  dbLocation,
}: {
  dbEngine: DatabaseEngine;
  dbLocation: DatabaseLocation;
}) {
  switch (dbLocation) {
    case DatabaseLocation.Aws:
      if (
        dbEngine === DatabaseEngine.Postgres ||
        dbEngine === DatabaseEngine.AuroraPostgres
      ) {
        return (
          <Box mb={3}>
            <Text mb={2}>
              Users must have an <Mark>rds_iam</Mark> role:
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
          </Box>
        );
      }
      if (
        dbEngine === DatabaseEngine.MySql ||
        dbEngine === DatabaseEngine.AuroraMysql
      ) {
        return (
          <Box mb={3}>
            <Box mb={2}>
              <Text mb={2}>
                Users must have the RDS authentication plugin enabled:
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
              <Text>
                Created user may not have access to anything by default so let's
                grant it some permissions:
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
          </Box>
        );
      }
      break;

    // self-hosted databases
    default:
      if (dbEngine === DatabaseEngine.Postgres) {
        return (
          <Box mb={3}>
            <Text mb={2}>
              Add the following entries to PostgreSQL's{' '}
              <Link
                href="https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html#UsingWithRDS.IAMDBAuth.DBAccounts.PostgreSQL"
                target="_blank"
              >
                host-based authentication
              </Link>{' '}
              file named <Mark>pg_hba.conf</Mark>, so that PostgreSQL requires
              client certificates from clients connecting over TLS:
            </Text>
            <TextSelectCopyMulti
              bash={false}
              lines={[
                {
                  text:
                    `hostssl all             all             ::/0                    cert\n` +
                    `hostssl all             all             0.0.0.0/0               cert\n`,
                },
              ]}
            />
            <Text mt={2}>
              Note: Ensure that you have no higher-priority md5 authentication
              rules that will match, otherwise PostgreSQL will offer them first,
              and the certificate-based Teleport login will fail.
            </Text>
          </Box>
        );
      }

      if (dbEngine === DatabaseEngine.MongoDb) {
        return (
          <Box mb={3}>
            <Text mb={2}>
              To create a user for this database, connect to this database using
              the <Mark>mongosh</Mark>
              or <Mark>mongo</Mark> shell and run the following command:
            </Text>
            <TextSelectCopyMulti
              bash={false}
              lines={[
                {
                  text:
                    `db.getSiblingDB("$external").runCommand(\n` +
                    `  {\n` +
                    `    createUser: "CN=YOUR_USERNAME",\n` +
                    `    roles: [\n` +
                    `      { role: "readWriteAnyDatabase", db: "admin" }\n` +
                    `    ]\n` +
                    `  }\n` +
                    `)`,
                },
              ]}
            />
          </Box>
        );
      }

      if (dbEngine === DatabaseEngine.MySql) {
        return (
          <Box mb={3}>
            <Text mb={2}>
              MySQL/MariaDB database user accounts must be configured to require
              a valid client certificate.
            </Text>
            <Box mb={2}>
              <Text bold>To create a new user:</Text>
              <TextSelectCopyMulti
                bash={false}
                lines={[
                  {
                    text: `CREATE USER 'YOUR_USERNAME'@'%' REQUIRE SUBJECT '/CN=YOUR_USERNAME';`,
                  },
                ]}
              />
            </Box>
            <Box mb={3}>
              <Text bold>To update an existing user:</Text>
              <TextSelectCopyMulti
                bash={false}
                lines={[
                  {
                    text: `ALTER USER 'YOUR_USERNAME'@'%' REQUIRE SUBJECT '/CN=YOUR_USERNAME';`,
                  },
                ]}
              />
            </Box>
            <Box>
              <Text>
                By default, the created user may not have access to anything and
                won't be able to connect, so let's grant it some permissions:
              </Text>
              <TextSelectCopyMulti
                bash={false}
                lines={[
                  {
                    text: "GRANT ALL ON `%`.* TO 'YOUR_USERNAME'@'%';",
                  },
                ]}
              />
            </Box>
          </Box>
        );
      }
  }

  return null;
}

// If auto discovery was enabled, users need to see all supported engine info
// to help setup access to their databases.
const AutoDiscoverInfoTabs = ({ location }: { location: DatabaseLocation }) => {
  return (
    <Tabs
      tabs={[
        {
          title: 'PostgreSQL',
          content: (
            <Box p={3}>
              <Info dbEngine={DatabaseEngine.Postgres} dbLocation={location} />
            </Box>
          ),
        },
        {
          title: `MySQL`,
          content: (
            <Box p={3}>
              <Info dbEngine={DatabaseEngine.MySql} dbLocation={location} />
            </Box>
          ),
        },
      ]}
    />
  );
};
