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

import React, { useCallback, useMemo } from 'react';
import styled, { css, keyframes } from 'styled-components';

import { NavLink, useLocation } from 'react-router-dom';

import { ExternalLinkIcon } from 'design/SVGIcon';

import { getIcon } from 'teleport/Navigation/utils';
import { NavigationDropdown } from 'teleport/Navigation/NavigationDropdown';
import {
  commonNavigationItemStyles,
  LinkContent,
  NavigationItemSize,
} from 'teleport/Navigation/common';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { storageService } from 'teleport/services/storageService';
import { useTeleport } from 'teleport';

import { NavTitle, RecommendationStatus } from 'teleport/types';
import { NotificationKind } from 'teleport/stores/storeNotifications';

import type {
  TeleportFeature,
  TeleportFeatureNavigationItem,
} from 'teleport/types';

interface NavigationItemProps {
  feature: TeleportFeature;
  size: NavigationItemSize;
  transitionDelay: number;
  visible: boolean;
}

const ExternalLink = styled.a`
  ${commonNavigationItemStyles};

  &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const highlight = keyframes`
  to {
    background: none;
  }
`;

const Link = styled(NavLink)`
  ${commonNavigationItemStyles};
  color: ${props => props.theme.colors.text.main};
  z-index: 1;
  background: ${p =>
    p.isHighlighted ? p.theme.colors.highlightedNavigationItem : 'none'};
  animation: ${p =>
    p.isHighlighted
      ? css`
          ${highlight} 10s forwards linear
        `
      : 'none'};
  animation-delay: 2s;

  &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }

  &.active {
    background: ${props => props.theme.colors.spotBackground[0]};
    border-left-color: ${props => props.theme.colors.brand};

    ${LinkContent} {
      font-weight: 700;
      opacity: 1;
    }
  }
`;

const ExternalLinkIndicator = styled.div`
  position: absolute;
  top: 50%;
  right: 18px;
  line-height: 0;
  transform: translate(0, -50%);
`;

export function NavigationItem(props: NavigationItemProps) {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();

  const {
    navigationItem,
    route,
    isLocked,
    lockedNavigationItem,
    lockedRoute,
    hideFromNavigation,
  } = props.feature;

  const { search } = useLocation();

  const params = useMemo(() => new URLSearchParams(search), [search]);

  const highlighted =
    props.feature.highlightKey &&
    params.get('highlight') === props.feature.highlightKey;

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      switch (event.key) {
        case 'ArrowDown':
          let nextSibling = event.currentTarget.nextSibling as HTMLDivElement;
          if (!nextSibling) {
            return;
          }

          if (nextSibling.nodeName !== 'A' && nextSibling.nodeName !== 'DIV') {
            nextSibling = nextSibling.nextSibling as HTMLDivElement;
          }

          if (nextSibling) {
            nextSibling.focus();
          }

          break;

        case 'ArrowUp':
          let previousSibling = event.currentTarget
            .previousSibling as HTMLDivElement;
          if (!previousSibling) {
            return;
          }

          // navigating up to a dropdown
          if (previousSibling.hasAttribute('data-open')) {
            const isOpen = previousSibling.getAttribute('data-open') === 'true';

            if (isOpen) {
              // focus on the last item in the open dropdown
              const lastLinkInDropdown =
                previousSibling.lastElementChild as HTMLAnchorElement;

              lastLinkInDropdown.focus();

              return;
            }

            // go to the previous sibling of the dropdown links to focus on the dropdown
            // container
            const dropdownContainer =
              previousSibling.previousSibling as HTMLDivElement;

            dropdownContainer.focus();

            return;
          }

          if (
            previousSibling.nodeName !== 'A' &&
            previousSibling.nodeName !== 'DIV'
          ) {
            previousSibling = previousSibling.previousSibling as HTMLDivElement;
          }

          if (previousSibling) {
            previousSibling.focus();
          }

          break;
      }
    },
    []
  );

  if (hideFromNavigation) {
    return null;
  }

  // renderHighlightFeature returns red dot component if the feature recommendation state is 'NOTIFY'
  function renderHighlightFeature(featureName: NavTitle): JSX.Element {
    if (featureName === NavTitle.AccessLists) {
      const hasNotifications = ctx.storeNotifications.hasNotificationsByKind(
        NotificationKind.AccessList
      );

      if (hasNotifications) {
        return <AttentionDot />;
      }

      return null;
    }

    // Get onboarding status. We'll only recommend features once user completes
    // initial onboarding (i.e. connect resources to Teleport cluster).
    const onboard = storageService.getOnboardDiscover();
    if (!onboard?.hasResource) {
      return null;
    }

    const recommendFeatureStatus =
      storageService.getFeatureRecommendationStatus();
    if (
      featureName === NavTitle.TrustedDevices &&
      recommendFeatureStatus?.TrustedDevices === RecommendationStatus.Notify
    ) {
      return <AttentionDot />;
    }
    return null;
  }

  if (navigationItem) {
    const linkProps = {
      style: {
        transitionDelay: `${props.transitionDelay}ms,0s`,
        transform: `translate3d(${
          props.visible ? 0 : 'calc(var(--sidebar-width) * -1)'
        }, 0, 0)`,
      },
    };

    if (navigationItem.isExternalLink) {
      return (
        <ExternalLink
          {...linkProps}
          onKeyDown={handleKeyDown}
          tabIndex={props.visible ? 0 : -1}
          href={navigationItem.getLink(clusterId)}
          target="_blank"
          rel="noopener noreferrer"
        >
          <LinkContent size={props.size}>
            {getIcon(props.feature, props.size)}
            {navigationItem.title}

            <ExternalLinkIndicator>
              <ExternalLinkIcon size={14} />
            </ExternalLinkIndicator>
          </LinkContent>
        </ExternalLink>
      );
    }

    let navigationItemVersion: TeleportFeatureNavigationItem;
    if (route) {
      navigationItemVersion = navigationItem;
    }

    // use locked item version if feature is locked
    if (lockedRoute && isLocked?.(ctx.lockedFeatures)) {
      if (!lockedNavigationItem) {
        throw new Error(
          'locked feature without an alternative navigation item'
        );
      }
      navigationItemVersion = lockedNavigationItem;
    }

    if (navigationItemVersion) {
      return (
        <Link
          {...linkProps}
          onKeyDown={handleKeyDown}
          tabIndex={props.visible ? 0 : -1}
          to={navigationItemVersion.getLink(clusterId)}
          exact={navigationItemVersion.exact}
          isHighlighted={highlighted}
        >
          <LinkContent size={props.size}>
            {getIcon(props.feature, props.size)}
            {navigationItemVersion.title}
            {renderHighlightFeature(props.feature.navigationItem.title)}
          </LinkContent>
        </Link>
      );
    }
  }

  return (
    <NavigationDropdown
      feature={props.feature}
      size={props.size}
      transitionDelay={props.transitionDelay}
      visible={props.visible}
    />
  );
}

const AttentionDot = styled.div.attrs(() => ({
  'data-testid': 'nav-item-attention-dot',
}))`
  margin-left: 15px;
  margin-top: 2px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: ${props => props.theme.colors.error.main};
`;
