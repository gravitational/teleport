/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { ReactNode, useEffect, useRef, useState } from 'react';
import { matchPath } from 'react-router';
import { NavLink } from 'react-router-dom';
import styled from 'styled-components';

import { ButtonIcon, Flex, H4, P3, Text } from 'design';
import { Cross } from 'design/Icon';

import { useFeatures } from 'teleport/FeaturesContext';
import { TeleportFeature } from 'teleport/types';

import { SidenavCategory } from './categories';
import { getSubsectionStyles } from './Section';

export type RecentHistoryItem = {
  category?: SidenavCategory;
  title: string;
  route: string;
  exact?: boolean;
};

type AnimatedItem = RecentHistoryItem & {
  animationState: 'exiting' | 'entering' | '';
};

function getIconForRoute(
  features: TeleportFeature[],
  route: string
): (props) => ReactNode {
  const feature = features.find(feature =>
    matchPath(route, {
      path: feature?.route?.path,
      exact: false,
    })
  );

  const icon = feature?.navigationItem?.icon || feature?.topMenuItem?.icon;
  if (!icon) {
    return () => null;
  }

  return icon;
}

export function RecentHistory({
  recentHistoryItems,
  onRemoveItem,
}: {
  recentHistoryItems: RecentHistoryItem[];
  onRemoveItem: (route: string) => void;
}) {
  const features = useFeatures();
  const [animatedItems, setAnimatedItems] = useState<AnimatedItem[]>([]);
  const prevItemsRef = useRef<RecentHistoryItem[]>([]);

  useEffect(() => {
    const prevItems = prevItemsRef.current;
    let newAnimatedItems = recentHistoryItems.map(item => ({
      ...item,
      animationState: '',
    })) as AnimatedItem[];

    const isFirstItemDeleted =
      recentHistoryItems.findIndex(
        item => item.route === prevItems[0]?.route
      ) === -1;

    // If an item the previous list is not in the new list (deleted) OR was moved, animate it out.
    prevItems.forEach((prevItem, index) => {
      if (
        !recentHistoryItems.some(item => item.route === prevItem.route) ||
        (prevItem?.route !== recentHistoryItems[index]?.route &&
          recentHistoryItems[0]?.route === prevItem?.route)
      ) {
        // If the item is now in the first position (meaning it was moved to the top), animate it in at the top in addition to animating it out in its previous position.
        if (
          recentHistoryItems.length > 0 &&
          prevItems[0]?.route !== recentHistoryItems[0]?.route &&
          !isFirstItemDeleted
        ) {
          newAnimatedItems.splice(0, 1);
          newAnimatedItems = [
            { ...prevItem, animationState: 'entering' },
            ...newAnimatedItems.slice(0, index),
            { ...prevItem, animationState: 'exiting' },
            ...newAnimatedItems.slice(index),
          ];
        } else if (
          !recentHistoryItems.some(item => item.route === prevItem.route)
        ) {
          newAnimatedItems = [
            ...newAnimatedItems.slice(0, index),
            { ...prevItem, animationState: 'exiting' },
            ...newAnimatedItems.slice(index),
          ];
        }
      }
    });

    setAnimatedItems(newAnimatedItems);
    prevItemsRef.current = recentHistoryItems;

    // Clean up animated items after animation
    const deletedItemTimeout = setTimeout(() => {
      setAnimatedItems(items =>
        items.filter(item => item.animationState !== 'exiting')
      );
    }, 300);
    const newItemsTimeout = setTimeout(() => {
      setAnimatedItems(items =>
        items.map(item => ({ ...item, animationState: '' }))
      );
    }, 400);

    return () => {
      clearTimeout(deletedItemTimeout);
      clearTimeout(newItemsTimeout);
    };
  }, [recentHistoryItems]);

  return (
    <Flex flexDirection="column" mt={3} width="100%" paddingRight={'3px'}>
      <H4 px={3} mb={1} color="text.muted" style={{ textTransform: 'none' }}>
        Recent Pages
      </H4>
      {!!animatedItems.length && (
        <Flex flexDirection="column">
          {animatedItems.map((item, index) => {
            const Icon = getIconForRoute(features, item.route);
            return (
              <AnimatedHistoryItem
                key={item.route + index}
                item={item}
                Icon={Icon}
                onRemove={() => onRemoveItem(item.route)}
              />
            );
          })}
        </Flex>
      )}
    </Flex>
  );
}

function AnimatedHistoryItem({
  item,
  Icon,
  onRemove,
}: {
  item: AnimatedItem;
  Icon: (props) => ReactNode;
  onRemove: () => void;
}) {
  const [hovered, setHovered] = useState(false);
  const itemRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (item.animationState === 'exiting' && itemRef.current) {
      const height = item.category ? 60 : 40;
      itemRef.current.style.height = `${height}px`;
      itemRef.current.style.opacity = '1';
      void itemRef.current.offsetHeight; // Force reflow
      requestAnimationFrame(() => {
        if (itemRef.current) {
          itemRef.current.style.height = '0px';
          itemRef.current.style.opacity = '0';
        }
      });
    }

    if (item.animationState === 'entering' && itemRef.current) {
      const height = item.category ? 60 : 40;
      itemRef.current.style.height = `0px`;
      itemRef.current.style.opacity = '0';
      void itemRef.current.offsetHeight; // Force reflow
      requestAnimationFrame(() => {
        if (itemRef.current) {
          itemRef.current.style.height = `${height}px`;
          itemRef.current.style.opacity = '1';
        }
      });
    }
  }, [item.animationState]);

  return (
    <AnimatedItemWrapper
      ref={itemRef}
      isExiting={item.animationState === 'exiting'}
      isEntering={item.animationState === 'entering'}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onMouseOver={() => setHovered(true)}
      style={{ height: item.animationState === 'entering' ? 0 : 'auto' }}
    >
      <StyledNavLink to={item.route}>
        <Flex width="100%" gap={2} alignItems="start">
          <Flex height="24px" alignItems="center" justifyContent="center">
            <Icon size={20} color="text.slightlyMuted" />
          </Flex>
          <Flex flexDirection="column" alignItems="start">
            <Text
              typography="body2"
              color="text.slightlyMuted"
              style={{
                textOverflow: 'ellipsis',
                overflow: 'hidden',
                whiteSpace: 'nowrap',
                width: hovered ? '115px' : '100%',
              }}
            >
              {item.title}
            </Text>
            {item.category && <P3 color="text.muted">{item.category}</P3>}
          </Flex>
        </Flex>
        {hovered && (
          <DeleteButtonAlt
            size={item.category ? 60 : 40}
            onMouseDown={e => {
              e.preventDefault();
              e.stopPropagation();
            }}
            onClick={e => {
              e.stopPropagation();
              e.preventDefault();
              onRemove();
            }}
          >
            <Cross size="large" color="text.primary" />
          </DeleteButtonAlt>
        )}
      </StyledNavLink>
    </AnimatedItemWrapper>
  );
}

const AnimatedItemWrapper = styled.div<{
  isExiting: boolean;
  isEntering: boolean;
}>`
  overflow: hidden;
  height: auto;
  width: 100%;
  transition: all 0.3s ease-in-out;
  padding: 3px;

  ${props =>
    props.isEntering &&
    `
    transition: all 0.3s ease-in-out 0.1s;
    pointer-events: none;
    opacity: 0;
  `}

  ${props =>
    props.isExiting &&
    `
    pointer-events: none;
  `}
`;

const StyledNavLink = styled(NavLink)`
  padding: ${props => props.theme.space[2]}px ${props => props.theme.space[3]}px;
  text-decoration: none;
  user-select: none;
  border-radius: ${props => props.theme.radii[2]}px;
  max-width: 100%;
  display: flex;
  position: relative;

  cursor: pointer;

  ${props => getSubsectionStyles(props.theme, false)}
`;

const DeleteButtonAlt = styled(ButtonIcon)<{ size: number }>`
  position: absolute;
  right: 0;
  top: 0;
  height: ${props => props.size}px;
  width: ${props => props.size}px;
  border-radius: ${props => props.theme.radii[2]}px;
`;
