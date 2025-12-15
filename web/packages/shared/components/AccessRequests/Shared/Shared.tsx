/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ComponentType, useState } from 'react';

import { Box, ButtonIcon, ButtonPrimary, Menu, Text } from 'design';
import { displayDateWithPrefixedTime } from 'design/datetime';
import { Info } from 'design/Icon';
import * as Icon from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest';
import { formattedName } from 'shared/components/AccessRequests/ReviewRequests';
import { AccessRequest } from 'shared/services/accessRequests';

export function PromotedMessage({
  request,
  px,
  py,
  self,
  assumeAccessList,
}: {
  request: AccessRequest;
  self: boolean;
  px?: number;
  py?: number;
  assumeAccessList(): void;
}) {
  const { promotedAccessListTitle, user } = request;

  return (
    <Box px={px} py={py}>
      <Text>
        This access request has been promoted to long-term access.
        <br />
        {self ? (
          <>
            You are now a member of Access List <b>{promotedAccessListTitle}</b>{' '}
            which grants you the resources requested.
          </>
        ) : (
          <>
            {user} is now a member of Access List{' '}
            <b>{promotedAccessListTitle}</b> which grants {user} the resources
            requested.
          </>
        )}
      </Text>
      {self && (
        <ButtonPrimary mt={3} onClick={assumeAccessList}>
          Re-login to gain access
        </ButtonPrimary>
      )}
    </Box>
  );
}

export const ButtonPromotedInfo = ({
  request,
  ownRequest,
  assumeAccessList,
}: {
  request: AccessRequest;
  ownRequest: boolean;
  assumeAccessList(): void;
}) => {
  const [anchorEl, setAnchorEl] = useState(null);

  const handleOpen = event => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  return (
    <Box css={{ margin: '0 auto' }}>
      <ButtonIcon onClick={handleOpen}>
        <Info />
      </ButtonIcon>
      <Menu
        anchorOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
      >
        <PromotedMessage
          request={request}
          self={ownRequest}
          assumeAccessList={assumeAccessList}
          px={4}
          py={4}
        />
      </Menu>
    </Box>
  );
};

export function getAssumeStartTimeTooltipText(startTime: Date) {
  const formattedDate = displayDateWithPrefixedTime(startTime);
  return `Access is not available until the approved time of ${formattedDate}`;
}

export const BlockedByStartTimeButton = ({
  assumeStartTime,
}: {
  assumeStartTime: Date;
}) => {
  return (
    <HoverTooltip
      tipContent={getAssumeStartTimeTooltipText(assumeStartTime)}
      placement="top-end"
    >
      <ButtonPrimary disabled={true} size="small">
        Assume Roles
      </ButtonPrimary>
    </HoverTooltip>
  );
};

/** Returns a list of requested roles or resources. */
export function getResourcesOrRolesFromRequest(
  request: Pick<AccessRequest, 'resources' | 'roles'>
): { Icon: ComponentType<IconProps>; name: string; title: string }[] {
  return request.resources.length
    ? request.resources.map(c => {
        const name = formattedName(c);
        return {
          Icon: getIcon(c.id.kind),
          name,
          title: `${c.id.kind}: ${name}`,
        };
      })
    : request.roles.map(c => ({
        Icon: getIcon('role'),
        name: c,
        title: `role: ${c}`,
      }));
}

function getIcon(item: RequestableResourceKind): ComponentType<IconProps> {
  switch (item) {
    case 'app':
    case 'saml_idp_service_provider':
    case 'aws_ic_account_assignment':
      return Icon.Application;
    case 'node':
      return Icon.Server;
    case 'db':
      return Icon.Database;
    case 'kube_cluster':
    case 'namespace':
      return Icon.Kubernetes;
    case 'role':
      return Icon.ClipboardUser;
    case 'user_group':
      return Icon.Server;
    case 'git_server':
      return Icon.GitHub;
    case 'windows_desktop':
      return Icon.Desktop;
    case 'resource': // This probably never shows in the UI.
      return Icon.Server;
    default:
      item satisfies never;
  }
}
