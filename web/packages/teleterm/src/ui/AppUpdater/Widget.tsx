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

import { Stack, Text } from 'design';
import { Alert } from 'design/Alert';
import { ShieldWarning } from 'design/Icon';
import { getErrorMessage } from 'shared/utils/error';

import type { AppUpdateEvent } from 'teleterm/services/appUpdater';
import { formatMB, iconMac } from 'teleterm/ui/AppUpdater/common';

export function Widget(props: {
  update: AppUpdateEvent;
  onChangeFlow(): void;
  onInstall?: () => void;
}) {
  // const { updateEvent } = useAppUpdaterContext();
  const updateEvent = props.update;
  if (
    updateEvent.kind === 'update-not-available' ||
    updateEvent.kind === 'checking-for-update'
  ) {
    return;
  }

  if (!updateEvent.update?.version) {
    return;
  }

  const { description, button } = getSmallContent(updateEvent, props.onInstall);

  return (
    <Alert
      icon={AsIcon}
      kind="neutral"
      justifyContent="space-between"
      alignItems="center"
      width="100%"
      mb={0}
      // py={1}
      // px={2}
      // onClick={props.onChangeFlow}
      details={
        <Stack gap={0}>
          <Text>{description}</Text>
          <Text typography={'body3'}>
            <ShieldWarning size="small" color={'yellow'} /> There are some
            issues.
          </Text>
        </Stack>
      }
      primaryAction={
        button ? { content: button.name, onClick: button.action } : undefined
      }
      secondaryAction={{ content: 'More', onClick: props.onChangeFlow }}
    >
      Teleport Connect {updateEvent.update.version}
      {/*<Stack width="100%">*/}
      {/*  /!*<Text>Managed Update</Text>*!/*/}
      {/*  <Flex width="100%" alignItems="center" justifyContent="space-between">*/}
      {/*    <Flex gap={1} alignItems="center" width="100%">*/}
      {/*      /!*<img height="50px" src={iconMac} />*!/*/}
      {/*      <Stack gap={0} maxWidth="100%" minWidth={0}>*/}
      {/*        <Text bold>Teleport Connect {updateEvent.update.version}</Text>*/}
      {/*        <P3*/}
      {/*          color="text.slightlyMuted"*/}
      {/*          css={`*/}
      {/*            max-width: 100%;*/}
      {/*            min-width: 0;*/}
      {/*            white-space: nowrap;*/}
      {/*          `}*/}
      {/*        >*/}
      {/*          {getSmallContent(updateEvent)}*/}
      {/*        </P3>*/}
      {/*      </Stack>*/}
      {/*    </Flex>*/}
      {/*    <ButtonBorder*/}
      {/*      onClick={props.onChangeFlow}*/}
      {/*      css={`*/}
      {/*        flex-shrink: 0;*/}
      {/*      `}*/}
      {/*      size="small"*/}
      {/*    >*/}
      {/*      Details*/}
      {/*    </ButtonBorder>*/}
      {/*    {updateEvent.kind === 'update-downloaded' && (*/}
      {/*      <ButtonPrimary*/}
      {/*        css={`*/}
      {/*          flex-shrink: 0;*/}
      {/*        `}*/}
      {/*        size="small"*/}
      {/*      >*/}
      {/*        Restart Now*/}
      {/*      </ButtonPrimary>*/}
      {/*    )}*/}
      {/*  </Flex>*/}
      {/*</Stack>*/}
    </Alert>
  );
}

function AsIcon() {
  return (
    <img
      height="50px"
      src={iconMac}
      css={`
        margin: -5px;
      `}
    />
  );
}

function getSmallContent(
  update: AppUpdateEvent,
  onInstall: () => void
): {
  description: string;
  button?: {
    name: string;
    action(): void;
  };
} {
  switch (update.kind) {
    case 'checking-for-update':
      return {
        description: 'Checking for updates…',
      };
    case 'download-progress':
      return {
        description: `Downloaded ${formatMB(update.progress.transferred)} of ${formatMB(update.progress.total)}`,
      };
    case 'update-not-available':
      return {
        description: 'Update not available',
      };
    case 'update-available':
      return {
        description: 'Update available. Starting download…',
      };
    case 'update-downloaded':
      return {
        description: 'Update downloaded',
        button: {
          name: 'Restart',
          action() {
            onInstall();
          },
        },
      };
    case 'error':
      return {
        description: `⚠ ${getErrorMessage(update.error)}`,
      };
  }
}
