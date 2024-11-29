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
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import { AuthType } from 'teleport/services/user';
import { generateTshLoginCommand } from 'teleport/lib/util';

export default function ConnectDialog({
  username,
  clusterId,
  organization,
  onClose,
  authType,
  accessRequestId,
}: Props) {
  let repoURL = `https://github.com/orgs/${organization}/repositories`;
  let title = `Use 'git' for GitHub Organization '${organization}'`;
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
        <DialogTitle>{title}</DialogTitle>
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
          {' - Connect'}
          <br />
          {'To clone a new repository, find the SSH url of the repository on '}
          <Link href={repoURL} target="_blank">
            github.com
          </Link>
          {' then'}
          <TextSelectCopy mt="2" text={`tsh git clone <git-clone-ssh-url>`} />
          {'To configure an existing Git repository, go to the repository then'}
          <TextSelectCopy mt="2" text={`tsh git config update`} />
        </Box>
        <Box>
          {`Once the repository is cloned or configured, use 'git' as normal.`}
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

export type Props = {
  organization: string;
  onClose: () => void;
  username: string;
  clusterId: string;
  authType: AuthType;
  accessRequestId?: string;
};
