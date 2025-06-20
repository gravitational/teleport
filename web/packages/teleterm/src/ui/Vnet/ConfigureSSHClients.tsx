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

import { Alert, ButtonIcon, ButtonPrimary, Flex, H2, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';
import { Cross } from 'design/Icon';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import { useAsync } from 'shared/hooks/useAsync';

export function ConfigureSSHClients(props: {
  onClose: () => void;
  onConfirm: () => Promise<void>;
  vnetSSHConfigPath: string;
  host?: string;
  hidden?: boolean;
}) {
  const [confirmAttempt, onConfirm] = useAsync(props.onConfirm);
  const host = props.host || '<hostname>.<cluster>';
  return (
    <Dialog
      open={!props.hidden}
      dialogCss={() => ({
        maxWidth: '800px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" alignItems="baseline">
        <H2>Configure SSH clients for VNet</H2>
        <ButtonIcon
          type="button"
          onClick={props.onClose}
          color="text.slightlyMuted"
        >
          <Cross size="medium" />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={4}>
        <Text typography="body2">
          Compatible SSH clients can connect to SSH servers with Teleport
          features like per-session MFA without complex configuration. Teleport
          VNet will handle the connection and manage SSH certificates
          automatically, simply connect with
        </Text>
        <TextSelectCopy text={`ssh <username>@${host}`} mt={2} />
        <Text typography="body2" mt={2}>
          To enable this for any OpenSSH-compatible client that reads
          configuration from <code>~/.ssh/config</code> add the following line
          at the top of the file or click the button below to add it
          automatically.
        </Text>
        <TextSelectCopy
          text={`Include "${props.vnetSSHConfigPath}"`}
          bash={false}
          mt={2}
        />
      </DialogContent>
      <DialogFooter>
        <Flex>
          <ButtonPrimary
            ml="auto"
            mr={1}
            onClick={() =>
              onConfirm().then(([, err]) => !err && props.onClose())
            }
            disabled={confirmAttempt.status === 'processing'}
          >
            Configure Automatically
          </ButtonPrimary>
        </Flex>
        {confirmAttempt.status === 'error' && (
          <Alert
            mt={4}
            mb={0}
            dismissible
            children={confirmAttempt.statusText}
          />
        )}
      </DialogFooter>
    </Dialog>
  );
}
