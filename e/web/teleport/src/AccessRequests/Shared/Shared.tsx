import React, { useState } from 'react';
import { ButtonPrimary, Text, Box, ButtonIcon, Menu } from 'design';
import { Info } from 'design/Icon';
import { format } from 'date-fns';
import { HoverTooltip } from 'shared/components/ToolTip';
import cfg from 'shared/config';

import { AccessRequest } from 'e-teleport/services/accessRequests';

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
  const formattedDate = format(startTime, cfg.dateWithPrefixedTime);
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
      anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
      transformOrigin={{ vertical: 'bottom', horizontal: 'right' }}
    >
      <ButtonPrimary disabled={true} size="small">
        Assume Roles
      </ButtonPrimary>
    </HoverTooltip>
  );
};
