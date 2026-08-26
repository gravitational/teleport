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

import { formatDistanceStrict } from 'date-fns';

import { ButtonSecondary, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import { ResetToken } from 'teleport/services/user';

export default function UserTokenLink({
  token,
  onClose,
  asInvite = false,
}: Props) {
  const tokenUrl = cfg.getUserResetTokenRoute(token.value, asInvite);
  const expiresText = formatDistanceStrict(Date.now(), token.expires);

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={close}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Share Link</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {asInvite ? (
          <Text mb={4} mt={1}>
            User
            <Text bold as="span">
              {` ${token.username} `}
            </Text>
            has been created but requires a password. Share this URL with the
            user to set up a password, link is valid for {expiresText}.
          </Text>
        ) : (
          <Text mb={4} mt={1}>
            User
            <Text bold as="span">
              {` ${token.username} `}
            </Text>
            has been reset. Share this URL with the user to set up a new
            password, link is valid for {expiresText}.
          </Text>
        )}
        <TextSelectCopy text={tokenUrl} bash={false} />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  token: ResetToken;
  onClose(): void;
  asInvite?: boolean;
};
