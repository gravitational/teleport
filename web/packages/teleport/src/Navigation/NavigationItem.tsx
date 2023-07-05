/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useCallback } from 'react';
import styled from 'styled-components';

import { NavLink } from 'react-router-dom';

import { ExternalLinkIcon } from 'design/SVGIcon';

import { getIcon } from 'teleport/Navigation/utils';
import { NavigationDropdown } from 'teleport/Navigation/NavigationDropdown';
import {
  commonNavigationItemStyles,
  LinkContent,
  NavigationItemSize,
} from 'teleport/Navigation/common';

import useStickyClusterId from 'teleport/useStickyClusterId';

import { useTeleport } from 'teleport';

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
    background: rgba(255, 255, 255, 0.05);
  }
`;

const Link = styled(NavLink)`
  ${commonNavigationItemStyles};

  &:focus {
    background: rgba(255, 255, 255, 0.05);
  }

  &.active {
    background: rgba(255, 255, 255, 0.05);
    border-left-color: #512fc9;

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

  const { navigationItem, route, isLocked, lockedNavigationItem, lockedRoute } =
    props.feature;

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
        >
          <LinkContent size={props.size}>
            {getIcon(props.feature, props.size)}
            {navigationItemVersion.title}
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
