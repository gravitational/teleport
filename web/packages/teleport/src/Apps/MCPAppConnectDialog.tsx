/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Box, ButtonSecondary, Stack, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import { generateTshLoginCommand } from 'teleport/lib/util';
import { App } from 'teleport/services/apps';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

/**
 * MCPAppConnectDialog shows tsh instructions for connecting to an MCP app.
 */
export function MCPAppConnectDialog(props: { app: App; onClose: () => void }) {
  const { app, onClose } = props;
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const { username, authType } = ctx.storeUser.state;
  const accessRequestId = ctx.storeUser.getAccessRequestId();

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
        <DialogTitle>Connect to: {app.name}</DialogTitle>
      </DialogHeader>

      <DialogContent>
        <Stack gap={4}>
          <Stack fullWidth gap={2}>
            <Text>
              <Text bold as="span">
                Step 1
              </Text>
              {' - Log in to Teleport'}
            </Text>
            <TextSelectCopy
              text={generateTshLoginCommand({
                authType,
                username,
                clusterId,
                accessRequestId,
              })}
            />
          </Stack>

          <Stack fullWidth gap={2}>
            <Text>
              <Text bold as="span">
                Step 2
              </Text>
              {' - Log in the MCP server'}
            </Text>
            <TextSelectCopy text={`tsh mcp login ${app.name}`} />
          </Stack>
          <Box>
            Restart your AI client to load the updated configuration if
            necessary.
          </Box>
        </Stack>
      </DialogContent>

      <DialogFooter>
        <ButtonSecondary onClick={props.onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
