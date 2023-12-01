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

import React, { Suspense, useState, lazy } from 'react';
import styled, { useTheme } from 'styled-components';
import { Flex, Text, TopNav } from 'design';

import { matchPath, useHistory } from 'react-router';

import { BrainIcon, OpenAIIcon } from 'design/SVGIcon';

import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { UserMenuNav } from 'teleport/components/UserMenuNav';
import { useFeatures } from 'teleport/FeaturesContext';

import cfg from 'teleport/config';

import { useLayout } from 'teleport/Main/LayoutContext';

import { KeysEnum } from 'teleport/services/storageService';

import {
  Popup,
  PopupButton,
  PopupFooter,
  PopupLogos,
  PopupLogosSpacer,
  PopupTitle,
  PopupTitleBackground,
  TeleportIcon,
} from 'teleport/Assist/Popup/Popup';

import ClusterSelector from './ClusterSelector';
import { Notifications } from './Notifications';
import { ButtonIconContainer } from './Shared';

const Assist = lazy(() => import('teleport/Assist'));

const AssistButtonContainer = styled.div`
  position: relative;
`;

const Background = styled.div`
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 98;
  background: rgba(0, 0, 0, 0.6);
`;

type TopBarProps = {
  // hidePopup indicates if the popup should be hidden based on parent component states.
  // if true, another modal is present; and we do not want to display the assist popup.
  // if false or absent, display as pre normal logical rules.
  hidePopup?: boolean;
};

export function TopBar({ hidePopup = false }: TopBarProps) {
  const theme = useTheme();

  const ctx = useTeleport();
  const history = useHistory();
  const features = useFeatures();

  const assistEnabled = ctx.getFeatureFlags().assist && ctx.assistEnabled;

  const [showAssistPopup, setShowAssistPopup] = useLocalStorage(
    KeysEnum.SHOW_ASSIST_POPUP,
    assistEnabled
  );

  const [showAssist, setShowAssist] = useState(false);

  const { clusterId, hasClusterUrl } = useStickyClusterId();

  const { hasDockedElement } = useLayout();

  function loadClusters() {
    return ctx.clusterService.fetchClusters();
  }

  function changeCluster(value: string) {
    const newPrefix = cfg.getClusterRoute(value);

    const oldPrefix = cfg.getClusterRoute(clusterId);

    const newPath = history.location.pathname.replace(oldPrefix, newPrefix);

    // TODO (avatus) DELETE IN 15 (LEGACY RESOURCES SUPPORT)
    // this is a temporary hack to support leaf clusters _maybe_ not having access
    // to unified resources yet. When unified resources are loaded in fetchUnifiedResources,
    // if the response is a 404 (the endpoint doesnt exist), we:
    // 1. push them to the servers page (old default)
    // 2. set this variable conditionally render the "legacy" navigation
    // When we switch clusters (to leaf or root), we remove the item and perform the check again by pushing
    // to the resource (new default view).
    window.localStorage.removeItem(KeysEnum.UNIFIED_RESOURCES_NOT_SUPPORTED);
    // we also need to reset the pinned resources flag when we switch clusters to try again
    window.localStorage.removeItem(KeysEnum.PINNED_RESOURCES_NOT_SUPPORTED);
    const legacyResourceRoutes = [
      cfg.getNodesRoute(clusterId),
      cfg.getAppsRoute(clusterId),
      cfg.getKubernetesRoute(clusterId),
      cfg.getDatabasesRoute(clusterId),
      cfg.getDesktopsRoute(clusterId),
    ];

    if (
      legacyResourceRoutes.some(route =>
        history.location.pathname.includes(route)
      )
    ) {
      const unifiedPath = cfg
        .getUnifiedResourcesRoute(clusterId)
        .replace(oldPrefix, newPrefix);

      history.replace(unifiedPath);
      return;
    }

    // keep current view just change the clusterId
    history.push(newPath);
  }

  // find active feature
  const feature = features
    .filter(feature => Boolean(feature.route))
    .find(f =>
      matchPath(history.location.pathname, {
        path: f.route.path,
        exact: f.route.exact ?? false,
      })
    );

  const title = feature?.route?.title || '';

  // instead of re-creating an expensive react-select component,
  // hide/show it instead
  const styles = {
    display: !hasClusterUrl ? 'none' : 'block',
  };

  return (
    <TopBarContainer>
      {!hasClusterUrl && (
        <Text fontSize="18px" bold data-testid="title">
          {title}
        </Text>
      )}
      <Text fontSize="18px" id="topbar-portal" ml={2}></Text>
      <ClusterSelector
        value={clusterId}
        width="384px"
        maxMenuHeight={200}
        mr="20px"
        onChange={changeCluster}
        onLoad={loadClusters}
        style={styles}
      />
      <Flex ml="auto" height="100%" alignItems="center">
        {!hasDockedElement && assistEnabled && (
          <AssistButtonContainer>
            <ButtonIconContainer onClick={() => setShowAssist(true)}>
              <BrainIcon />
            </ButtonIconContainer>
            {showAssistPopup && !hidePopup && (
              <>
                <Background />
                <Popup data-testid="assistPopup">
                  <PopupTitle>
                    <PopupTitleBackground>New!</PopupTitleBackground>
                  </PopupTitle>{' '}
                  Try out Teleport Assist, a GPT-4-powered AI assistant that
                  leverages your infrastructure
                  <PopupFooter>
                    <PopupLogos>
                      <OpenAIIcon size={30} />
                      <PopupLogosSpacer>+</PopupLogosSpacer>
                      <TeleportIcon light={theme.type === 'light'} />
                    </PopupLogos>

                    <PopupButton onClick={() => setShowAssistPopup(false)}>
                      Close
                    </PopupButton>
                  </PopupFooter>
                </Popup>
              </>
            )}
          </AssistButtonContainer>
        )}
        <Notifications />
        <UserMenuNav username={ctx.storeUser.state.username} />
      </Flex>

      {showAssist && (
        <Suspense fallback={null}>
          <Assist onClose={() => setShowAssist(false)} />
        </Suspense>
      )}
    </TopBarContainer>
  );
}

export const TopBarContainer = styled(TopNav)`
  height: 72px;
  background-color: inherit;
  padding-left: ${({ theme }) => `${theme.space[6]}px`};
  overflow-y: initial;
  flex-shrink: 0;
  border-bottom: 1px solid ${({ theme }) => theme.colors.spotBackground[0]};
`;
