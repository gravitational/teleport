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

import { WelcomeWrapper } from 'teleport/components/Onboard';
import {
  Route,
  Switch,
  useLocation,
  useParams,
} from 'teleport/components/Router';
import cfg from 'teleport/config';
import history from 'teleport/services/history';
import { NewCredentialsContainerProps } from 'teleport/Welcome/NewCredentials';

import { CardWelcome } from './CardWelcome';
import { CLOUD_INVITE_URL_PARAM } from './const';

type WelcomeProps = {
  NewCredentials: (props: NewCredentialsContainerProps) => JSX.Element;
};

export function Welcome({ NewCredentials }: WelcomeProps) {
  const { tokenId } = useParams<{ tokenId: string }>();
  const { search } = useLocation();

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

  return (
    <Switch>
      <Route exact path={cfg.routes.userInvite}>
        <WelcomeWrapper>
          <CardWelcome
            title="Welcome to Teleport"
            subTitle="Please click the button below to create an account"
            btnText="Get started"
            onClick={handleOnInviteContinue}
          />
        </WelcomeWrapper>
      </Route>
      <Route exact path={cfg.routes.userReset}>
        <WelcomeWrapper>
          <CardWelcome
            title="Reset Authentication"
            subTitle="Please click the button below to begin recovery of your account"
            btnText="Continue"
            onClick={handleOnResetContinue}
          />
        </WelcomeWrapper>
      </Route>
      <Route path={cfg.routes.userInviteContinue}>
        <WelcomeWrapper>
          <NewCredentials tokenId={tokenId} />
        </WelcomeWrapper>
      </Route>
      <Route path={cfg.routes.userResetContinue}>
        <WelcomeWrapper>
          <NewCredentials resetMode={true} tokenId={tokenId} />
        </WelcomeWrapper>
      </Route>
    </Switch>
  );
}
