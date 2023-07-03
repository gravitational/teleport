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

import React, { useState } from 'react';
import styled from 'styled-components';
import { Text, Box, Flex, Link } from 'design';
import { Danger } from 'design/Alert';
import { InfoFilled } from 'design/Icon';
import TextEditor from 'shared/components/TextEditor';

import useTeleport from 'teleport/useTeleport';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { Tabs } from 'teleport/components/Tabs';

import { HeaderSubtitle, ActionButtons, Mark, Header } from '../../Shared';
import { dbCU } from '../../yamlTemplates';
import { DatabaseEngine } from '../../SelectResource';

import { useMutualTls, State } from './useMutualTls';

import type { AgentStepProps } from '../../types';

export function MutualTls(props: AgentStepProps) {
  const ctx = useTeleport();
  const state = useMutualTls({ ctx, props });

  return <MutualTlsView {...state} />;
}

export function MutualTlsView({
  attempt,
  onNextStep,
  curlCmd,
  canUpdateDatabase,
  dbEngine,
}: State) {
  const [caCert, setCaCert] = useState('');

  return (
    <Box maxWidth="800px">
      <Header>Configure Mutual TLS</Header>
      <HeaderSubtitle>
        Self-hosted databases must be configured with Teleport's certificate
        authority to be able to verify client certificates. They also need a
        certificate/key pair that Teleport can verify.
      </HeaderSubtitle>
      {attempt.status === 'failed' && <Danger children={attempt.statusText} />}
      {!canUpdateDatabase && (
        <Box>
          <Text>
            You don't have permission to update a database.
            <br />
            Please ask your Teleport administrator to update your role and add
            the <Mark>db</Mark> rule:
          </Text>
          <Flex minHeight="195px" mt={3}>
            <TextEditor
              readOnly={true}
              bg="levels.deep"
              data={[{ content: dbCU, type: 'yaml' }]}
            />
          </Flex>
        </Box>
      )}
      {canUpdateDatabase && (
        <>
          <Box mb={3}>
            <Text bold>
              Run the command below to download Teleport's CA and generate
              cert/key pair.
            </Text>
            <Box mt={2} mb={1}>
              <TextSelectCopyMulti lines={[{ text: curlCmd }]} />
            </Box>
          </Box>
          <StyledBox mb={6}>
            <Flex mb={2}>
              <InfoFilled fontSize={18} mr={1} mt="2px" />
              <Text bold>After Running the Command</Text>
            </Flex>
            <DbEngineInstructions dbEngine={dbEngine} />
          </StyledBox>
          <Box mb={5}>
            <Text bold>Add a copy of your CA certificate*</Text>
            <Text mb={2}>
              *Only required if your database is configured with a certificate
              signed by a third-party CA. Adding a copy allows Teleport to trust
              it.
            </Text>
            <Box
              mt={2}
              height="100px"
              width="800px"
              as="textarea"
              p={2}
              borderRadius={2}
              placeholder="Copy and paste your CA certificate"
              value={caCert}
              onChange={e => setCaCert(e.target.value)}
              autoFocus
              style={{ outline: 'none' }}
            />
          </Box>
        </>
      )}
      <ActionButtons
        onProceed={() => onNextStep(caCert)}
        disableProceed={attempt.status === 'processing'}
      />
    </Box>
  );
}

function DbEngineInstructions({ dbEngine }: { dbEngine: DatabaseEngine }) {
  if (dbEngine === DatabaseEngine.Postgres) {
    return (
      <Box>
        <Text mb={1}>
          Add the following to the PostgreSQL configuration file{' '}
          <Mark>postgresql.conf</Mark>, to have your server accept{' '}
          <Link
            href="https://www.postgresql.org/docs/current/ssl-tcp.html"
            target="_blank"
          >
            TLS connections
          </Link>
          :
        </Text>
        <TextSelectCopyMulti
          bash={false}
          lines={[
            {
              text:
                `ssl = on\n` +
                `ssl_cert_file = '$PGDATA/server.crt'\n` +
                `ssl_key_file = '$PGDATA/server.key'\n` +
                `ssl_ca_file = '$PGDATA/server.cas'`,
            },
          ]}
        />
      </Box>
    );
  }

  if (dbEngine === DatabaseEngine.MongoDb) {
    return (
      <Box>
        <Text mb={3}>
          Use the generated secrets to{' '}
          <Link
            href="https://www.mongodb.com/docs/manual/tutorial/configure-ssl/"
            target="_blank"
          >
            enable mutual TLS
          </Link>{' '}
          in your
          <Mark>mongod.conf</Mark> configuration file and restart the database:
        </Text>
        <Tabs
          tabs={[
            {
              title: 'MongoDB 3.6 - 4.2',
              content: (
                <TextSelectCopyMulti
                  bash={false}
                  lines={[
                    {
                      text:
                        `net:\n` +
                        `  ssl:\n` +
                        `    mode: requireSSL\n` +
                        `    PEMKeyFile: /etc/certs/mongo.crt\n` +
                        `    CAFile: /etc/certs/mongo.cas`,
                    },
                  ]}
                />
              ),
            },
            {
              title: 'MongoDB 4.2+',
              content: (
                <TextSelectCopyMulti
                  bash={false}
                  lines={[
                    {
                      text:
                        `net:\n` +
                        `  tls:\n` +
                        `    mode: requireTLS\n` +
                        `    certificateKeyFile: /etc/certs/mongo.crt\n` +
                        `    CAFile: /etc/certs/mongo.cas`,
                    },
                  ]}
                />
              ),
            },
          ]}
        />
      </Box>
    );
  }

  if (dbEngine === DatabaseEngine.MySql) {
    return (
      <Box>
        <Text mb={3}>
          To configure this database to accept TLS connections, add the
          following to your configuration file, <Mark>mysql.cnf</Mark>:
        </Text>
        <Tabs
          tabs={[
            {
              title: 'MySQL',
              content: (
                <>
                  <TextSelectCopyMulti
                    bash={false}
                    lines={[
                      {
                        text:
                          `[mysqld]\n` +
                          `require_secure_transport=ON\n` +
                          `ssl-ca=/path/to/server.cas\n` +
                          `ssl-cert=/path/to/server.crt\n` +
                          `ssl-key=/path/to/server.key`,
                      },
                    ]}
                  />
                  <Text mt={2}>
                    See{' '}
                    <Link
                      href="https://dev.mysql.com/doc/refman/8.0/en/using-encrypted-connections.html"
                      target="_blank"
                    >
                      Configuring MySQL to Use Encrypted Connections
                    </Link>{' '}
                    for more details.
                  </Text>
                </>
              ),
            },
            {
              title: 'MariaDB',
              content: (
                <>
                  <TextSelectCopyMulti
                    bash={false}
                    lines={[
                      {
                        text:
                          `[mariadb]\n` +
                          `require_secure_transport=ON\n` +
                          `ssl-ca=/path/to/server.cas\n` +
                          `ssl-cert=/path/to/server.crt\n` +
                          `ssl-key=/path/to/server.key`,
                      },
                    ]}
                  />
                  <Text mt={2}>
                    See{' '}
                    <Link
                      href="https://mariadb.com/docs/server/security/data-in-transit-encryption/enterprise-server/enable-tls/"
                      target="_blank"
                    >
                      Enabling TLS on MariaDB Server
                    </Link>{' '}
                    for more details.
                  </Text>
                </>
              ),
            },
          ]}
        />
      </Box>
    );
  }
}

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 8px;
  padding: 20px;
`;
