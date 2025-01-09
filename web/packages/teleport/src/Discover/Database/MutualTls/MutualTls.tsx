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

import { useState } from 'react';

import { Box, Flex, Link, Mark, Text } from 'design';
import { Danger } from 'design/Alert';
import { Info } from 'design/Icon';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import TextEditor from 'shared/components/TextEditor';
import Validation from 'shared/components/Validation';

import { Tabs } from 'teleport/components/Tabs';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import useTeleport from 'teleport/useTeleport';

import { DatabaseEngine } from '../../SelectResource';
import { ActionButtons, Header, HeaderSubtitle, StyledBox } from '../../Shared';
import type { AgentStepProps } from '../../types';
import { dbCU } from '../../yamlTemplates';
import { State, useMutualTls } from './useMutualTls';

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
              <Info size="medium" mr={1} />
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
            <Validation>
              <FieldTextArea
                mt={2}
                placeholder="Copy and paste your CA certificate"
                value={caCert}
                onChange={e => setCaCert(e.target.value)}
                resizable={true}
                autoFocus
                textAreaCss={`
                height: 100px;
                width: 800px;
                `}
              />
            </Validation>
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
        <RestartDatabaseText link="https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/postgres-self-hosted/#step-25-create-a-certificatekey-pair" />
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
                  <RestartDatabaseText link="https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/mysql-self-hosted/#step-24-create-a-certificatekey-pair" />
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
                  <RestartDatabaseText link="https://goteleport.com/docs/enroll-resources/database-access/enroll-self-hosted-databases/mysql-self-hosted/#step-24-create-a-certificatekey-pair" />
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

const RestartDatabaseText = ({ link }: { link: string }) => (
  <Text mt={1}>
    Restart the database server to apply the configuration. The certificate is
    valid for 90 days so this will require installing an{' '}
    <Link href={link} target="_blank">
      updated certificate
    </Link>{' '}
    and restarting the database server before that to continue access.
  </Text>
);
