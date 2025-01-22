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

import { Box, ButtonSecondary, Link, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import { generateTshLoginCommand } from 'teleport/lib/util';
import { AuthType } from 'teleport/services/user';

export function ConnectDialog({
  username,
  clusterId,
  organization,
  onClose,
  authType,
  accessRequestId,
}: {
  organization: string;
  onClose: () => void;
  username: string;
  clusterId: string;
  authType: AuthType;
  accessRequestId?: string;
}) {
  const repoURL = `https://github.com/orgs/${organization}/repositories`;
  const title = `Use 'git' for GitHub Organization '${organization}'`;
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
      <DialogContent gap={4}>
        <Box>
          <Text bold as="span">
            Step 1
          </Text>
          {' - Log in to Teleport'}
          <TextSelectCopy
            mt="1"
            mb="2"
            text={generateTshLoginCommand({
              authType,
              clusterId,
              username,
              accessRequestId,
            })}
          />
        </Box>
        <Box>
          <Text bold as="span">
            Step 2
          </Text>
          {' - Clone or configure a repository'}
          <br />
          {'To clone a new repository, find the SSH url of the repository on '}
          <Link href={repoURL} target="_blank">
            github.com
          </Link>
          {', and then'}
          <TextSelectCopy
            mt="1"
            mb="2"
            text="tsh git clone <git-clone-ssh-url>"
          />
          To configure an existing Git repository, go to the repository and then
          <TextSelectCopy mt="1" mb="2" text="tsh git config update" />
        </Box>
        <Box>
          Once the repository is cloned or configured, use 'git' as normal.
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
