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

import { Box, ButtonSecondary, Flex, Link, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { ResourceIcon } from 'design/ResourceIcon';
import { getDatabaseIconName } from 'shared/components/UnifiedResources/shared/viewItemsFactory';
import { DbProtocol } from 'shared/services/databases';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { generateTshLoginCommand } from 'teleport/lib/util';
import { AuthType } from 'teleport/services/user';

export default function ConnectDialog({
  username,
  clusterId,
  dbName,
  onClose,
  authType,
  accessRequestId,
  dbProtocol,
}: Props) {
  // For dynamodb and clickhouse-http protocols, the command is `tsh proxy db --tunnel` instead of `tsh db connect`.
  let connectCommand =
    dbProtocol == 'dynamodb' || dbProtocol == 'clickhouse-http'
      ? 'proxy db --tunnel'
      : 'db connect';

  // Adjust `--db-name` flag based on db protocol, as it's required for
  // some, optional for some, and unsupported by some.
  let dbNameFlag: string;
  switch (dbProtocol) {
    case 'postgres':
    case 'sqlserver':
    case 'oracle':
    case 'mongodb':
    case 'spanner':
      // Required
      dbNameFlag = ' --db-name=<name>';
      break;
    case 'cassandra':
    case 'clickhouse':
    case 'clickhouse-http':
    case 'dynamodb':
    case 'opensearch':
    case 'elasticsearch':
    case 'redis':
      // No flag
      dbNameFlag = '';
      break;
    default:
      // Default to optional
      dbNameFlag = ' [--db-name=<name>]';
  }

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
        <DialogTitle>
          <Flex gap={2}>
            Connect to:
            <Flex gap={1}>
              <ResourceIcon
                name={getDatabaseIconName(dbProtocol)}
                width="24px"
                height="24px"
              />
              {dbName}
            </Flex>
          </Flex>
        </DialogTitle>
      </DialogHeader>
      <DialogContent minHeight="240px" flex="0 0 auto">
        <Box mb={4}>
          <Text bold as="span">
            Step 1
          </Text>
          {' - Login to Teleport'}
          <TextSelectCopy
            mt="2"
            text={generateTshLoginCommand({
              authType,
              clusterId,
              username,
              accessRequestId,
            })}
          />
        </Box>
        <Box mb={4}>
          <Text bold as="span">
            Step 2
          </Text>
          {' - Connect to the database'}
          <TextSelectCopy
            mt="2"
            text={`tsh ${connectCommand} ${dbName} --db-user=<user>${dbNameFlag}`}
          />
        </Box>
        {accessRequestId && (
          <Box mb={4}>
            <Text bold as="span">
              Step 3 (Optional)
            </Text>
            {' - When finished, drop the assumed role'}
            <TextSelectCopy mt="2" text={`tsh request drop`} />
          </Box>
        )}
        <Box>
          {`* Note: To connect with a GUI database client, see our `}
          <Link
            href={
              'https://goteleport.com/docs/connect-your-client/gui-clients/'
            }
            target="_blank"
          >
            documentation
          </Link>
          {` for instructions.`}
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

export type Props = {
  dbName: string;
  dbProtocol: DbProtocol;
  onClose: () => void;
  username: string;
  clusterId: string;
  authType: AuthType;
  accessRequestId?: string;
};
