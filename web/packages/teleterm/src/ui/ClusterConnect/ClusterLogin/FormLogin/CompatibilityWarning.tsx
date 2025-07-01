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

import { Flex, Indicator, Stack } from 'design';
import { ActionButton, Warning } from 'design/Alert';
import { Download, NewTab } from 'design/Icon';
import {
  AuthSettings,
  ClientVersionStatus,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/auth_settings_pb';

import { Platform } from 'teleterm/mainProcess/types';
import { useAppUpdaterContext } from 'teleterm/ui/AppUpdater/AppUpdaterContext';
import { Widget } from 'teleterm/ui/AppUpdater/Widget';

export function CompatibilityWarning(props: {
  authSettings: AuthSettings;
  platform: Platform;
  shouldSkipVersionCheck: boolean;
  onChangeFlow: () => void;
  disableVersionCheck(): void;
  mx?: number;
}) {
  // if (props.shouldSkipVersionCheck) {
  //   return;
  // }

  const appUpdaterContext = useAppUpdaterContext();

  const warning = getWarning(props.authSettings);

  if (!warning) {
    return;
  }

  return (
    <Stack>
      <Flex px={4} width="100%">
        <Widget
          onChangeFlow={props.onChangeFlow}
          update={appUpdaterContext.updateEvent}
          onInstall={appUpdaterContext.quitAndInstall}
        />
      </Flex>
      {/*<Warning*/}
      {/*  mb={0}*/}
      {/*  mx={props.mx}*/}
      {/*  alignItems="flex-start"*/}
      {/*  details={*/}
      {/*    <Stack gap={2}>*/}
      {/*      <span>*/}
      {/*        Cluster teleport-17-ent.asteroid.earth requires Teleport Connect*/}
      {/*        17.3.4 but teleport-16-ent.asteroid.earth requires 16.3.3.*/}
      {/*        <br />*/}
      {/*        Version 16.x was selected as most compatible.*/}
      {/*      </span>*/}
      {/*      <Flex justifyContent="center" width="100%" gap={2}>*/}
      {/*        <ActionButton*/}
      {/*          fill="border"*/}
      {/*          intent="neutral"*/}
      {/*          inputAlignment*/}
      {/*          action={{*/}
      {/*            content: (*/}
      {/*              <>*/}
      {/*                Force Update to 17.3.4*/}
      {/*                <Download size="small" ml={1} />*/}
      {/*              </>*/}
      {/*            ),*/}
      {/*          }}*/}
      {/*        />*/}
      {/*      </Flex>*/}
      {/*    </Stack>*/}
      {/*  }*/}
      {/*>*/}
      {/*  Updates are managed by another cluster*/}
      {/*</Warning>*/}
      {/*<Flex px={4}>*/}
      {/*  <Content*/}
      {/*    updateEvent={{*/}
      {/*      kind: 'download-progress',*/}
      {/*      progress: { percent: 50 },*/}
      {/*      update: { version: '16.3.3' },*/}
      {/*    }}*/}
      {/*  />*/}
      {/*</Flex>*/}
    </Stack>
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
