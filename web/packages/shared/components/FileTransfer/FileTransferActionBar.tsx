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

import { ButtonIcon, Flex, Text } from 'design';
import * as Icons from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import { useFileTransferContext } from './FileTransferContextProvider';

type FileTransferActionBarProps = {
  isConnected: boolean;
  // if any role has `options.ssh_file_copy: true` without any other role having `false` (false overrides).
  hasAccess: boolean;
};

export function FileTransferActionBar({
  isConnected,
  hasAccess,
}: FileTransferActionBarProps) {
  const fileTransferContext = useFileTransferContext();
  const areFileTransferButtonsDisabled =
    fileTransferContext.openedDialog || !isConnected || !hasAccess;

  return (
    <Flex flex="none" alignItems="center" height="24px">
      <HoverTooltip
        position="bottom"
        tipContent={
          !hasAccess ? (
            <Text>
              You are missing the{' '}
              <Text bold as="span">
                ssh_file_copy
              </Text>{' '}
              role option.
            </Text>
          ) : null
        }
      >
        <ButtonIcon
          disabled={areFileTransferButtonsDisabled}
          size={0}
          title="Download files"
          onClick={fileTransferContext.openDownloadDialog}
        >
          <Icons.Download size={16} />
        </ButtonIcon>
        <ButtonIcon
          disabled={areFileTransferButtonsDisabled}
          size={0}
          title="Upload files"
          onClick={fileTransferContext.openUploadDialog}
        >
          <Icons.Upload size={16} />
        </ButtonIcon>
      </HoverTooltip>
    </Flex>
  );
}
