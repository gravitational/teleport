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

import React from 'react';
import { Text, Box, ButtonSecondary, Link } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { DbProtocol } from 'shared/services/databases';

import { AuthType } from 'teleport/services/user';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { generateTshLoginCommand } from 'teleport/lib/util';

export default function ConnectDialog({
  username,
  clusterId,
  dbName,
  onClose,
  authType,
  accessRequestId,
}: Props) {
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
        <DialogTitle>Connect To Database</DialogTitle>
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
          {' - Retrieve credentials for the database'}
          <TextSelectCopy
            mt="2"
            text={`tsh db login [--db-user=<user>] [--db-name=<name>] ${dbName}`}
          />
        </Box>
        <Box mb={4}>
          <Text bold as="span">
            Step 3
          </Text>
          {' - Connect to the database'}
          <TextSelectCopy
            mt="2"
            text={`tsh db connect [--db-user=<user>] [--db-name=<name>] ${dbName}`}
          />
        </Box>
        {accessRequestId && (
          <Box mb={4}>
            <Text bold as="span">
              Step 4 (Optional)
            </Text>
            {' - When finished, drop the assumed role'}
            <TextSelectCopy mt="2" text={`tsh request drop`} />
          </Box>
        )}
        <Box>
          {`* Note: To connect with a GUI database client, see our `}
          <Link
            href={
              'https://goteleport.com/docs/database-access/guides/gui-clients/'
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
