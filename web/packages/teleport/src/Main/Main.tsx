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

import React, {
  createContext,
  lazy,
  ReactNode,
  Suspense,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
} from 'react';
import styled from 'styled-components';
import { Box, Indicator } from 'design';
import { Failed } from 'design/CardError';

import useAttempt from 'shared/hooks/useAttemptNext';

import { matchPath, useHistory } from 'react-router';

import Dialog from 'design/Dialog';
import { sharedStyles } from 'design/theme/themes/sharedStyles';

import { AssistViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/assist_pb';

import { Redirect, Route, Switch } from 'teleport/components/Router';
import { CatchError } from 'teleport/components/CatchError';
import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';
import { TopBar } from 'teleport/TopBar';
import { BannerList } from 'teleport/components/BannerList';
import { storageService } from 'teleport/services/storageService';
import { ClusterAlert, LINK_LABEL } from 'teleport/services/alerts/alerts';
import { useAlerts } from 'teleport/components/BannerList/useAlerts';
import { FeaturesContextProvider, useFeatures } from 'teleport/FeaturesContext';
import {
  getFirstRouteForCategory,
  Navigation,
} from 'teleport/Navigation/Navigation';
import { NavigationCategory } from 'teleport/Navigation/categories';
import { TopBarProps } from 'teleport/TopBar/TopBar';
import { useUser } from 'teleport/User/UserContext';
import { QuestionnaireProps } from 'teleport/Welcome/NewCredentials';

import { MainContainer } from './MainContainer';
import { OnboardDiscover } from './OnboardDiscover';

import type { BannerType } from 'teleport/components/BannerList/BannerList';
import type { LockedFeatures, TeleportFeature } from 'teleport/types';

const Assist = lazy(() => import('teleport/Assist'));

export interface MainProps {
  initialAlerts?: ClusterAlert[];
  customBanners?: ReactNode[];
  features: TeleportFeature[];
  billingBanners?: ReactNode[];
  Questionnaire?: (props: QuestionnaireProps) => React.ReactElement;
  topBarProps?: TopBarProps;
  inviteCollaboratorsFeedback?: ReactNode;
}

export function Main(props: MainProps) {
  const ctx = useTeleport();
  const history = useHistory();

  const { attempt, setAttempt, run } = useAttempt('processing');

  const { preferences } = useUser();

  useEffect(() => {
    if (ctx.storeUser.state) {
      setAttempt({ status: 'success' });
      return;
    }

    run(() => ctx.init(preferences));
  }, []);

  const viewMode = preferences?.assist?.viewMode;
  const assistEnabled = ctx.getFeatureFlags().assist && ctx.assistEnabled;
  const [showAssist, setShowAssist] = useState(false);
  const featureFlags = ctx.getFeatureFlags();

  const features = useMemo(
    () => props.features.filter(feature => feature.hasAccess(featureFlags)),
    [featureFlags, props.features]
  );
  const feature = features
    .filter(feature => Boolean(feature.route))
    .find(f =>
      matchPath(history.location.pathname, {
        path: f.route.path,
        exact: f.route.exact ?? false,
      })
    );

  const { alerts, dismissAlert } = useAlerts(props.initialAlerts);

  // if there is a redirectUrl, do not show the onboarding popup - it'll get in the way of the redirected page
  const [showOnboardDiscover, setShowOnboardDiscover] = useState(
    !ctx.redirectUrl
  );
  const [showOnboardSurvey, setShowOnboardSurvey] = useState<boolean>(
    !!props.Questionnaire
  );

  useEffect(() => {
    if (
      matchPath(history.location.pathname, {
        path: ctx.redirectUrl,
        exact: true,
      })
    ) {
      // hide the onboarding popup if we're on the redirectUrl, just in case
      setShowOnboardDiscover(false);
      ctx.redirectUrl = null;
    }
  }, [ctx, history.location.pathname]);

  if (attempt.status === 'failed') {
    return <Failed message={attempt.statusText} />;
  }

  if (attempt.status !== 'success') {
    return (
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
    );
  }

  function handleOnboard() {
    updateOnboardDiscover();
    history.push(cfg.routes.discover);
  }

  function handleOnClose() {
    updateOnboardDiscover();
    setShowOnboardDiscover(false);
  }

  function updateOnboardDiscover() {
    const discover = storageService.getOnboardDiscover();
    storageService.setOnboardDiscover({ ...discover, notified: true });
  }

  // redirect to the default feature when hitting the root /web URL
  if (
    matchPath(history.location.pathname, { path: cfg.routes.root, exact: true })
  ) {
    if (ctx.redirectUrl) {
      return <Redirect to={ctx.redirectUrl} />;
    }

    const indexRoute = cfg.isDashboard
      ? cfg.routes.downloadCenter
      : getFirstRouteForCategory(features, NavigationCategory.Resources);

    return <Redirect to={indexRoute} />;
  }

  // The backend defines the severity as an integer value with the current
  // pre-defined values: LOW: 0; MEDIUM: 5; HIGH: 10
  const mapSeverity = (severity: number) => {
    if (severity < 5) {
      return 'info';
    }
    if (severity < 10) {
      return 'warning';
    }
    return 'danger';
  };

  const banners: BannerType[] = alerts.map(alert => ({
    message: alert.spec.message,
    severity: mapSeverity(alert.spec.severity),
    link: alert.metadata.labels[LINK_LABEL],
    id: alert.metadata.name,
  }));

  const onboard = storageService.getOnboardDiscover();
  const requiresOnboarding =
    onboard && !onboard.hasResource && !onboard.notified;
  const displayOnboardDiscover = requiresOnboarding && showOnboardDiscover;
  const hasSidebar =
    feature?.category === NavigationCategory.Management &&
    !feature?.hideNavigation;

  return (
    <FeaturesContextProvider value={features}>
      <TopBar
        CustomLogo={
          props.topBarProps?.showPoweredByLogo
            ? props.topBarProps.CustomLogo
            : null
        }
        assistProps={{
          showAssist,
          setShowAssist,
          assistEnabled,
        }}
      />
      <Wrapper>
        <MainContainer>
          <Navigation />
          <HorizontalSplit
            dockedView={showAssist && viewMode === AssistViewMode.DOCKED}
            hasSidebar={hasSidebar}
          >
            <ContentMinWidth>
              <BannerList
                banners={banners}
                customBanners={props.customBanners}
                billingBanners={featureFlags.billing && props.billingBanners}
                onBannerDismiss={dismissAlert}
              />
              <Suspense fallback={null}>
                <FeatureRoutes lockedFeatures={ctx.lockedFeatures} />
              </Suspense>
            </ContentMinWidth>
          </HorizontalSplit>

          {showAssist && (
            <Suspense fallback={null}>
              <Assist onClose={() => setShowAssist(false)} />
            </Suspense>
          )}
        </MainContainer>
      </Wrapper>
      {displayOnboardDiscover && (
        <OnboardDiscover onClose={handleOnClose} onOnboard={handleOnboard} />
      )}
      {showOnboardSurvey && (
        <Dialog open={showOnboardSurvey}>
          <props.Questionnaire
            onSubmit={() => setShowOnboardSurvey(false)}
            onboard={false}
          />
        </Dialog>
      )}
      {props.inviteCollaboratorsFeedback}
    </FeaturesContextProvider>
  );
}

function renderRoutes(
  features: TeleportFeature[],
  lockedFeatures: LockedFeatures
) {
  const routes = [];

  for (const [index, feature] of features.entries()) {
    const isParentLocked =
      feature.parent && new feature.parent().isLocked?.(lockedFeatures);

    // remove features with parents locked.
    // The parent itself will be rendered if it has a lockedRoute,
    // but the children shouldn't be.
    if (isParentLocked) {
      continue;
    }

    // add the route of the 'locked' variants of the features
    if (feature.isLocked?.(lockedFeatures)) {
      if (!feature.lockedRoute) {
        throw new Error('a locked feature without a locked route was found');
      }

      const { path, title, exact, component: Component } = feature.lockedRoute;
      routes.push(
        <Route title={title} key={index} path={path} exact={exact}>
          <CatchError>
            <Suspense fallback={null}>
              <Component />
            </Suspense>
          </CatchError>
        </Route>
      );

      // return early so we don't add the original route
      continue;
    }

    // add regular feature routes
    if (feature.route) {
      const { path, title, exact, component: Component } = feature.route;
      routes.push(
        <Route title={title} key={index} path={path} exact={exact}>
          <CatchError>
            <Suspense fallback={null}>
              <Component />
            </Suspense>
          </CatchError>
        </Route>
      );
    }
  }

  return routes;
}

function FeatureRoutes({ lockedFeatures }: { lockedFeatures: LockedFeatures }) {
  const features = useFeatures();
  const routes = renderRoutes(features, lockedFeatures);

  return <Switch>{routes}</Switch>;
}

// This context allows children components to disable this min-width in case they want to be able to shrink smaller.
type MinWidthContextState = {
  setEnforceMinWidth: (enforceMinWidth: boolean) => void;
};

const ContentMinWidthContext = createContext<MinWidthContextState>(null);

/**
 * @deprecated Use useNoMinWidth instead.
 */
export const useContentMinWidthContext = () =>
  useContext(ContentMinWidthContext);

export const useNoMinWidth = () => {
  const { setEnforceMinWidth } = useContext(ContentMinWidthContext);

  useLayoutEffect(() => {
    setEnforceMinWidth(false);

    return () => {
      setEnforceMinWidth(true);
    };
  }, []);
};

const ContentMinWidth = ({ children }: { children: ReactNode }) => {
  const [enforceMinWidth, setEnforceMinWidth] = useState(true);

  return (
    <ContentMinWidthContext.Provider value={{ setEnforceMinWidth }}>
      <div
        css={`
          display: flex;
          flex-direction: column;
          flex: 1;
          ${enforceMinWidth ? 'min-width: 1000px;' : ''}
        `}
      >
        {children}
      </div>
    </ContentMinWidthContext.Provider>
  );
};

function getWidth(hasSidebar: boolean, isDockedView: boolean) {
  const { dockedAssistWidth, sidebarWidth } = sharedStyles;
  if (hasSidebar && isDockedView) {
    return `max-width: calc(100% - ${sidebarWidth}px - ${dockedAssistWidth}px);`;
  }
  if (isDockedView) {
    return `max-width: calc(100% - ${dockedAssistWidth}px);`;
  }
  if (hasSidebar) {
    return `max-width: calc(100% - ${sidebarWidth}px);`;
  }
  return 'max-width: 100%;';
}

export const HorizontalSplit = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  ${props => getWidth(props.hasSidebar, props.dockedView)}
  overflow-x: auto;
`;

export const StyledIndicator = styled(HorizontalSplit)`
  align-items: center;
  justify-content: center;
  position: absolute;
  overflow: hidden;
  top: 50%;
  left: 50%;
`;

const Wrapper = styled(Box)<{ hasDockedElement: boolean }>`
  display: flex;
  height: 100vh;
  flex-direction: column;
  width: ${p =>
    p.hasDockedElement
      ? `calc(100vw - ${p.theme.dockedAssistWidth}px)`
      : '100vw'};
`;
