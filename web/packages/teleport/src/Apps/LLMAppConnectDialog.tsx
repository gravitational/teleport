/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { ButtonSecondary, Stack, Text } from 'design';
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

export function LLMAppConnectDialog(props: { app: App; onClose: () => void }) {
  const { app } = props;
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
      onClose={props.onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>Use "{app.name}" inference endpoint</DialogTitle>
      </DialogHeader>

      <DialogContent>
        <Stack gap={3} fullWidth>
          <Stack fullWidth>
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

          <Stack fullWidth>
            <Text>
              <Text bold as="span">
                Step 2
              </Text>
              {' - Start a local proxy for the inference endpoint'}
            </Text>

            <TextSelectCopy text={`tsh proxy app ${app.name}`} />
          </Stack>

          <Stack>
            <Text>
              <Text bold as="span">
                Step 3
              </Text>
              {
                ' - Point your LLM client at the local proxy address printed by tsh'
              }
            </Text>
          </Stack>
        </Stack>
      </DialogContent>

      <DialogFooter>
        <ButtonSecondary onClick={props.onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
