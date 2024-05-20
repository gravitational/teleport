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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { ChevronRightIcon } from 'design/SVGIcon';

import { NavLink } from 'react-router-dom';

import { matchPath, useHistory } from 'react-router';

import {
  commonNavigationItemStyles,
  LinkContent,
  NavigationItemSize,
} from 'teleport/Navigation/common';
import { useFeatures } from 'teleport/FeaturesContext';
import { getIcon } from 'teleport/Navigation/utils';

import { useTeleport } from 'teleport';

import type { Location } from 'history';

import type { TeleportFeature } from 'teleport/types';

interface NavigationDropdownProps {
  feature: TeleportFeature;
  size: NavigationItemSize;
  transitionDelay: number;
  visible: boolean;
}

interface OpenProps {
  open: boolean;
}

const Container = styled.div<OpenProps>`
  ${commonNavigationItemStyles};

  cursor: pointer;

  &:focus {
    outline: none;
    background: ${props => props.theme.colors.spotBackground[0]};
  }

  ${LinkContent} {
    opacity: ${p => (p.open ? 0.9 : 0.6)};
  }
`;

const DropdownArrow = styled.div<OpenProps>`
  position: absolute;
  top: 50%;
  right: 18px;
  line-height: 0;
  transform: translate(0, -50%);

  svg {
    transform: ${p => (p.open ? 'rotate(90deg)' : 'none')};
    transition: 0.1s linear transform;
  }
`;

const DropdownLinks = styled.div<OpenProps>`
  overflow: hidden;
  max-height: ${p => (p.open ? 500 : 0)}px;
  transition: ${p =>
    p.open
      ? 'max-height 0.3s ease-in-out'
      : 'max-height 0.3s cubic-bezier(0, 1, 0, 1)'};
  transform: translate3d(0, 0, 0);
`;

const DropdownLink = styled(NavLink)`
  ${commonNavigationItemStyles};

  &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }

  &.active {
    background: ${props => props.theme.colors.spotBackground[1]};

    ${LinkContent} {
      font-weight: 700;
      opacity: 1;
    }

    &:before {
      height: 8px;
      width: 8px;
      position: absolute;
      top: 50%;
      transform: translate(0, -50%);
      left: 37px;
      border-radius: 2px;
      background: ${props => props.theme.colors.brand};
      content: '';
    }
  }
`;

function hasActiveChild(features: TeleportFeature[], route: Location<unknown>) {
  const feature = features
    .filter(feature => Boolean(feature.route))
    .find(feature =>
      matchPath(route.pathname, {
        path: feature.route.path,
        exact: false,
      })
    );

  return Boolean(feature);
}

export function NavigationDropdown(props: NavigationDropdownProps) {
  const features = useFeatures();
  const ctx = useTeleport();
  const history = useHistory();

  const ref = useRef<HTMLDivElement>();
  const firstLinkRef = useRef<HTMLAnchorElement>();

  const clusterId = ctx.storeUser.getClusterId();

  const childFeatures = features
    .filter(feature => Boolean(feature.parent))
    .filter(feature => props.feature instanceof feature.parent);

  const [open, setOpen] = useState(
    hasActiveChild(childFeatures, history.location)
  );

  useEffect(() => {
    return history.listen(next => {
      setOpen(hasActiveChild(childFeatures, next));
    });
  }, []);

  useEffect(() => {
    if (!props.visible) {
      setOpen(false);
    }
  }, [props.visible]);

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      switch (event.key) {
        case 'ArrowRight':
          setOpen(true);

          break;

        case 'ArrowLeft':
          setOpen(false);

          break;

        case 'Enter':
          setOpen(open => !open);

          break;

        case 'ArrowUp':
          const previousSibling = event.currentTarget
            .previousSibling as HTMLAnchorElement;
          if (previousSibling) {
            previousSibling.focus();
          }

          break;

        case 'ArrowDown':
          if (open) {
            firstLinkRef.current.focus();

            return;
          }

          // nextSibling is `DropdownLinks`, so we need to go one further to get the next
          // navigation item
          const nextSibling = event.currentTarget.nextSibling
            .nextSibling as HTMLDivElement;
          if (nextSibling) {
            nextSibling.focus();
          }

          break;
      }
    },
    [firstLinkRef, open]
  );

  const handleKeyDownLink = useCallback(
    (event: React.KeyboardEvent) => {
      switch (event.key) {
        case 'ArrowDown':
          const nextSibling = event.currentTarget.nextSibling as HTMLDivElement;

          if (nextSibling) {
            nextSibling.focus();

            return;
          }

          const nextParentSibling = event.currentTarget.parentElement
            .nextSibling as HTMLDivElement;
          if (nextParentSibling) {
            nextParentSibling.focus();
          }

          break;

        case 'ArrowUp':
          const previousSibling = event.currentTarget
            .previousSibling as HTMLDivElement;
          if (previousSibling) {
            previousSibling.focus();

            return;
          }

          ref.current.focus();

          break;

        case 'ArrowLeft':
          setOpen(false);
          ref.current.focus();

          break;
      }
    },
    [ref]
  );

  const items = childFeatures.map((feature, index) => (
    <DropdownLink
      ref={index === 0 ? firstLinkRef : null}
      onKeyDown={handleKeyDownLink}
      tabIndex={open ? 0 : -1}
      style={{
        transitionDelay: `${props.transitionDelay}ms,0s`,
        transform: `translate3d(${
          props.visible ? 0 : 'calc(var(--sidebar-width) * -1)'
        }, 0, 0)`,
      }}
      to={feature.navigationItem.getLink(clusterId)}
      key={index}
    >
      <LinkContent size={NavigationItemSize.Indented}>
        {getIcon(feature, NavigationItemSize.Small)}
        {feature.navigationItem.title}
      </LinkContent>
    </DropdownLink>
  ));

  return (
    <>
      <Container
        ref={ref}
        tabIndex={props.visible ? 0 : -1}
        onClick={() => setOpen(!open)}
        onKeyDown={handleKeyDown}
        open={open}
        style={{
          transitionDelay: `${props.transitionDelay}ms,0s`,
          transform: `translate3d(${
            props.visible ? 0 : 'calc(var(--sidebar-width) * -1)'
          }, 0, 0)`,
        }}
      >
        <LinkContent size={props.size}>
          {getIcon(props.feature, props.size)}

          {props.feature.navigationItem.title}

          <DropdownArrow open={open}>
            <ChevronRightIcon />
          </DropdownArrow>
        </LinkContent>
      </Container>

      <DropdownLinks data-open={open} open={open}>
        {items}
      </DropdownLinks>
    </>
  );
}
