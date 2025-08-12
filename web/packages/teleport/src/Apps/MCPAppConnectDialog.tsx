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

import {
  Box,
  ButtonSecondary,
  Flex,
  Link,
  ResourceIcon,
  Stack,
  Text,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import {
  TextSelectCopy,
  TextSelectCopyMulti,
} from 'shared/components/TextSelectCopy';
import {
  generateClaudeDesktopConfigForApp,
  generateInstallLinksForApp,
} from 'shared/services/mcp';

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
  const claudeConfig = generateClaudeDesktopConfigForApp(app.name);
  const links = generateInstallLinksForApp(app.name);

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
              {' - Configure your MCP client'}
            </Text>
            <Flex alignItems="center" justifyContent="left" columnGap={2}>
              <Link href={links.cursor} target="_blank">
                <ResourceIcon name="mcpCursor" height="32px" />
              </Link>
              <Link href={links.vscode} target="_blank">
                <ResourceIcon name="mcpVscode" height="25px" />
              </Link>
              <Link href={links.vscodeInsiders} target="_blank">
                <ResourceIcon name="mcpVscodeInsiders" height="25px" />
              </Link>
            </Flex>
            <Box>
              Here is a sample Claude Desktop config to connect to this MCP
              server:
            </Box>
            <TextSelectCopyMulti
              bash={false}
              lines={[
                {
                  text: claudeConfig,
                },
              ]}
            />
            <Box>
              Alternatively, run the following to generate the config from the
              command line.
            </Box>
            <TextSelectCopy text={`tsh mcp config ${app.name}`} />
            <Box>
              Note: You might need to restart your MCP client to load the
              updated configuration.
            </Box>
          </Stack>
        </Stack>
      </DialogContent>

      <DialogFooter>
        <ButtonSecondary onClick={props.onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
