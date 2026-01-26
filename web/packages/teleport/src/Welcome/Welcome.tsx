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

import type { JSX } from 'react';
import { matchPath, useLocation, useParams } from 'react-router';

import { WelcomeWrapper } from 'teleport/components/Onboard';
import cfg from 'teleport/config';
import history from 'teleport/services/history';
import { NewCredentialsContainerProps } from 'teleport/Welcome/NewCredentials';

import { CardWelcome } from './CardWelcome';
import { CLOUD_INVITE_URL_PARAM } from './const';

type WelcomeProps = {
  NewCredentials: (props: NewCredentialsContainerProps) => JSX.Element;
};

export function Welcome({ NewCredentials }: WelcomeProps) {
  const { tokenId } = useParams() as { tokenId: string };
  const { search, pathname } = useLocation();

  const handleOnInviteContinue = () => {
    // We need to pass through the `invite` query parameter (if it exists) to
    // render the invite collaborators form for Cloud users.
    let suffix = '';
    if (new URLSearchParams(search).has(CLOUD_INVITE_URL_PARAM)) {
      suffix = `?${CLOUD_INVITE_URL_PARAM}`;
    }

    history.push(`${cfg.getUserInviteTokenContinueRoute(tokenId)}${suffix}`);
  };

  const handleOnResetContinue = () => {
    history.push(cfg.getUserResetTokenContinueRoute(tokenId));
  };

  // Use matchPath to determine which view to render.
  const isInvite = matchPath(cfg.routes.userInvite, pathname);
  const isReset = matchPath(cfg.routes.userReset, pathname);
  const isInviteContinue = matchPath(cfg.routes.userInviteContinue, pathname);
  const isResetContinue = matchPath(cfg.routes.userResetContinue, pathname);

  if (isInvite) {
    return (
      <WelcomeWrapper>
        <CardWelcome
          title="Welcome to Teleport"
          subTitle="Please click the button below to create an account"
          btnText="Get started"
          onClick={handleOnInviteContinue}
        />
      </WelcomeWrapper>
    );
  }

  if (isReset) {
    return (
      <WelcomeWrapper>
        <CardWelcome
          title="Reset Authentication"
          subTitle="Please click the button below to begin recovery of your account"
          btnText="Continue"
          onClick={handleOnResetContinue}
        />
      </WelcomeWrapper>
    );
  }

  if (isInviteContinue) {
    return (
      <WelcomeWrapper>
        <NewCredentials tokenId={tokenId} />
      </WelcomeWrapper>
    );
  }

  if (isResetContinue) {
    return (
      <WelcomeWrapper>
        <NewCredentials resetMode={true} tokenId={tokenId} />
      </WelcomeWrapper>
    );
  }

  // Fallback - shouldn't normally reach here if routes are configured correctly
  return null;
}
