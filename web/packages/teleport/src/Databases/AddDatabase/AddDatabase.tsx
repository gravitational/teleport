/**
 * Copyright 2021 Gravitational, Inc.
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

import React, { useState } from 'react';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import {
  Text,
  Box,
  ButtonSecondary,
  Link,
  ButtonLink,
  Indicator,
} from 'design';
import Select, { Option } from 'shared/components/Select';
import { AuthType } from 'teleport/services/user';
import { DbType, DbProtocol } from 'teleport/services/databases';
import {
  formatDatabaseInfo,
  DatabaseInfo,
} from 'teleport/services/databases/makeDatabase';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import DownloadLinks from 'teleport/components/DownloadLinks';
import useTeleport from 'teleport/useTeleport';
import useAddDatabase, { State } from './useAddDatabase';

export default function Container(props: Props) {
  const ctx = useTeleport();
  const state = useAddDatabase(ctx);
  return <AddDatabase {...state} {...props} />;
}

export function AddDatabase({
  createJoinToken,
  expiry,
  attempt,
  token,
  authType,
  username,
  onClose,
  isEnterprise,
  version,
}: Props & State) {
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;
  const [dbOptions] = useState<Option<DatabaseInfo>[]>(() =>
    options.map(dbOption => {
      return {
        value: dbOption,
        label: dbOption.title,
      };
    })
  );

  const [selectedDbOption, setSelectedDbOption] = useState<
    Option<DatabaseInfo>
  >(dbOptions[0]);

  const connectCmd =
    authType === 'sso'
      ? `tsh login --proxy=${host}`
      : `tsh login --proxy=${host} --auth=local --user=${username}`;
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '600px',
        width: '100%',
      })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>Add Database</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'processing' && (
          <Box textAlign="center">
            <Indicator />
          </Box>
        )}
        {attempt.status === 'failed' && (
          <StepsWithoutToken
            loginCommand={connectCmd}
            addCommand={generateDbStartCmd(
              selectedDbOption.value.type,
              selectedDbOption.value.protocol,
              host,
              ''
            )}
            selectedDb={selectedDbOption}
            onDbChange={(o: Option<DatabaseInfo>) => setSelectedDbOption(o)}
            dbOptions={dbOptions}
            isEnterprise={isEnterprise}
            version={version}
          />
        )}
        {attempt.status === 'success' && (
          <StepsWithToken
            selectedDb={selectedDbOption}
            onDbChange={(o: Option<DatabaseInfo>) => setSelectedDbOption(o)}
            dbOptions={dbOptions}
            command={generateDbStartCmd(
              selectedDbOption.value.type,
              selectedDbOption.value.protocol,
              host,
              token
            )}
            expiry={expiry}
            onRegenerateToken={createJoinToken}
            isEnterprise={isEnterprise}
            version={version}
          />
        )}
      </DialogContent>
      {attempt.status !== 'processing' && (
        <DialogFooter>
          <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
        </DialogFooter>
      )}
    </Dialog>
  );
}

type StepsWithTokenProps = {
  selectedDb: Option<DatabaseInfo>;
  onDbChange: (o: Option<DatabaseInfo>) => void;
  dbOptions: Option<DatabaseInfo>[];
  command: string;
  expiry: string;
  onRegenerateToken: () => Promise<boolean>;
  isEnterprise: boolean;
  version: string;
};

const StepsWithToken = ({
  selectedDb,
  onDbChange,
  dbOptions,
  expiry,
  command,
  onRegenerateToken,
  isEnterprise,
  version,
}: StepsWithTokenProps) => (
  <>
    <Box mb={4}>
      <Text bold as="span">
        Step 1
      </Text>
      {' - Download Teleport package to your computer '}
      <DownloadLinks isEnterprise={isEnterprise} version={version} />
    </Box>
    <Box mb={4}>
      <Text bold as="span">
        Step 2
      </Text>
      {` - Select the database type and protocol to use`}
      <Box mt={2}>
        <Select
          value={selectedDb}
          onChange={onDbChange}
          options={dbOptions}
          isSearchable={true}
          maxMenuHeight={220}
        />
      </Box>
    </Box>
    <Box>
      <Text bold as="span">
        Step 3
      </Text>
      {' - Start the Teleport agent with the following parameters'}
      <Text mt="1">
        The token will be valid for{' '}
        <Text bold as={'span'}>
          {expiry}.
        </Text>
      </Text>
      <TextSelectCopy mt="2" text={command} />
    </Box>
    <Box>
      <ButtonLink onClick={onRegenerateToken}>Regenerate Token</ButtonLink>
    </Box>
    <Box mt={4}>
      {`Learn more about database access in our `}
      <Link
        href={'https://goteleport.com/docs/database-access/'}
        target="_blank"
      >
        documentation
      </Link>
      .
    </Box>
  </>
);

type StepsWithoutTokenProps = {
  selectedDb: Option<DatabaseInfo>;
  onDbChange: (o: Option<DatabaseInfo>) => void;
  dbOptions: Option<DatabaseInfo>[];
  isEnterprise: boolean;
  version: string;
  loginCommand: string;
  addCommand: string;
};

const StepsWithoutToken = ({
  loginCommand,
  addCommand,
  selectedDb,
  dbOptions,
  onDbChange,
  isEnterprise,
  version,
}: StepsWithoutTokenProps) => (
  <>
    <Box mb={4}>
      <Text bold as="span">
        Step 1
      </Text>
      {' - Download Teleport package to your computer '}
      <DownloadLinks isEnterprise={isEnterprise} version={version} />
    </Box>
    <Box mb={4}>
      <Text bold as="span">
        Step 2
      </Text>
      {' - Login to Teleport'}
      <TextSelectCopy mt="2" text={loginCommand} />
    </Box>
    <Box mb={4}>
      <Text bold as="span">
        Step 3
      </Text>
      {' - Generate a join token'}
      <TextSelectCopy mt="2" text="tctl tokens add --type=db" />
    </Box>
    <Box mb={4}>
      <Text bold as="span">
        Step 4
      </Text>
      {` - Select the database type and protocol to use`}
      <Box mt={2}>
        <Select
          value={selectedDb}
          onChange={onDbChange}
          options={dbOptions}
          isSearchable={true}
          maxMenuHeight={220}
        />
      </Box>
    </Box>
    <Box>
      <Text bold as="span">
        Step 5
      </Text>
      {' - Start the Teleport agent with the following parameters'}
      <TextSelectCopy mt="2" text={addCommand} />
    </Box>
    <Box mt={4}>
      {`Learn more about database access in our `}
      <Link
        href={'https://goteleport.com/docs/database-access/'}
        target="_blank"
      >
        documentation
      </Link>
      .
    </Box>
  </>
);

const generateDbStartCmd = (
  type: DbType,
  protocol: DbProtocol,
  host: string,
  token: string
) => {
  let baseCommand = `teleport db start --token=${
    token || '[generated-join-token]'
  } --auth-server=${host} --name=[db-name] --protocol=${protocol} --uri=[uri]`;

  if (protocol === 'sqlserver') {
    baseCommand =
      `${baseCommand} --ad-keytab-file=/path/to/teleport.keytab ` +
      `--ad-domain=EXAMPLE.COM ` +
      `--ad-spn=MSSQLSvc/sqlserver.example.com:1433`;
  }

  switch (type) {
    case 'self-hosted':
      return baseCommand;
    case 'rds':
      return `${baseCommand} --aws-region=[region]`;
    case 'redshift':
      return `${baseCommand} --aws-region=[region] --aws-redshift-cluster-id=[cluster-id]`;
    case 'gcp':
      return `${baseCommand} --ca-cert=[instance-ca-filepath] --gcp-project-id=[project-id] --gcp-instance-id=[instance-id]`;
    default:
      return 'unknown type and protocol';
  }
};

const options: DatabaseInfo[] = [
  formatDatabaseInfo('rds', 'postgres'),
  formatDatabaseInfo('rds', 'mysql'),
  formatDatabaseInfo('rds', 'sqlserver'),
  formatDatabaseInfo('redshift', 'postgres'),
  formatDatabaseInfo('gcp', 'postgres'),
  formatDatabaseInfo('gcp', 'mysql'),
  formatDatabaseInfo('gcp', 'sqlserver'),
  formatDatabaseInfo('self-hosted', 'postgres'),
  formatDatabaseInfo('self-hosted', 'mysql'),
  formatDatabaseInfo('self-hosted', 'mongodb'),
  formatDatabaseInfo('self-hosted', 'sqlserver'),
  formatDatabaseInfo('self-hosted', 'redis'),
];

export type Props = {
  isEnterprise: boolean;
  onClose(): void;
  username: string;
  version: string;
  authType: AuthType;
};
