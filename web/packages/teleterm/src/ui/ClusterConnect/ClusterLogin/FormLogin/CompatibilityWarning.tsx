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

import { Flex, Stack } from 'design';
import { ActionButton, Warning } from 'design/Alert';
import { NewTab } from 'design/Icon';
import {
  AuthSettings,
  ClientVersionStatus,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/auth_settings_pb';

import { Platform } from 'teleterm/mainProcess/types';

export function CompatibilityWarning(props: {
  authSettings: AuthSettings;
  platform: Platform;
  shouldSkipVersionCheck: boolean;
  disableVersionCheck(): void;
  mx?: number;
}) {
  if (props.shouldSkipVersionCheck) {
    return;
  }

  const warning = getWarning(props.authSettings);

  if (!warning) {
    return;
  }

  return (
    <Warning
      mb={0}
      mx={props.mx}
      alignItems="flex-start"
      details={
        <Stack gap={2}>
          {warning}
          <Flex justifyContent="space-between" width="100%" gap={2}>
            <ActionButton
              fill="border"
              intent="neutral"
              inputAlignment
              action={{
                content: (
                  <>
                    Download in Browser
                    <NewTab size="small" ml={1} />
                  </>
                ),
                href: buildDownloadUrl(props.platform),
              }}
            />
            <ActionButton
              fill="minimal"
              intent="neutral"
              inputAlignment
              action={{
                content: 'Do Not Show Again',
                onClick: props.disableVersionCheck,
              }}
            />
          </Flex>
        </Stack>
      }
    >
      Detected potentially incompatible version
    </Warning>
  );
}

function getWarning({
  clientVersionStatus,
  versions,
}: AuthSettings): string | undefined {
  switch (clientVersionStatus) {
    case ClientVersionStatus.TOO_OLD: {
      return (
        `Minimum Teleport Connect version supported by the server is ${versions.minClient} ` +
        `but you are using ${versions.client}.\nTo ensure full compatibility, please upgrade ` +
        `Teleport Connect to ${versions.minClient} or newer.`
      );
    }
    case ClientVersionStatus.TOO_NEW: {
      const serverVersionWithWildcards = `${getMajorVersion(versions.server)}.x.x`;
      return (
        `Maximum Teleport Connect version supported by the server is ${serverVersionWithWildcards} ` +
        `but you are using ${versions.client}.\nTo ensure full compatibility, please downgrade ` +
        `Teleport Connect to ${serverVersionWithWildcards}.`
      );
    }
    case ClientVersionStatus.OK:
    case ClientVersionStatus.COMPAT_UNSPECIFIED:
      return;
    default:
      clientVersionStatus satisfies never;
      return;
  }
}

function buildDownloadUrl(platform: Platform): string {
  let os: string;
  switch (platform) {
    case 'darwin':
      os = 'mac';
      break;
    case 'linux':
      os = 'linux';
      break;
    case 'win32':
      os = 'windows';
      break;
  }

  return `https://goteleport.com/download/?product=connect&os=${os}`;
}

function getMajorVersion(version: string): string {
  return version.split('.').at(0);
}
