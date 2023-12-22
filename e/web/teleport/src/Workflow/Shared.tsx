import React, { useState } from 'react';
import { ButtonPrimary, Text, Box, ButtonIcon, Menu } from 'design';
import { Info } from 'design/Icon';

import session from 'teleport/services/websession/websession';

import { AccessRequest } from 'e-teleport/services/workflow';
import TeleportContextE from 'e-teleport/teleportContextE';

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

export function getBaseRequestFlags(
  request: AccessRequest,
  ctx: TeleportContextE
) {
  const ownRequest = request.user === ctx.storeUser.getUsername();
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed =
    ownRequest && !!ctx.storeAccessRequests.isAssumed(request.id);

  const isPromoted = request.state === 'PROMOTED';

  return {
    // canAssume is a flag to show the assume btn.
    canAssume,
    // isAssumed is a flag if the assume btn should be disabled or not,
    // and determines the text that implies if user already has assumed or not.
    isAssumed,
    ownRequest,
    isPromoted,
  };
}

export function reloginWebUi(): void {
  session.logout();
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
