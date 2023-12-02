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

import { ChevronDownIcon } from 'design/SVGIcon/ChevronDown';

import { NavigationCategory } from 'teleport/Navigation/categories';

type NavigationItems = {
  category: NavigationCategory;
  requiresAttention?: boolean;
};

interface NavigationSwitcherProps {
  onChange: (value: NavigationCategory) => void;
  value: NavigationCategory;
  items: NavigationItems[];
}

interface OpenProps {
  open: boolean;
}

interface ActiveProps {
  active: boolean;
}

const Container = styled.div`
  position: relative;
  align-self: center;
  user-select: none;
  margin-bottom: 25px;
  margin-top: 26px;
`;

const ActiveValue = styled.div<OpenProps>`
  border: 1px solid ${props => props.theme.colors.text.slightlyMuted};
  border-radius: 4px;
  padding: 12px 16px;
  width: 190px;
  box-sizing: border-box;
  position: relative;
  cursor: pointer;

  &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const Dropdown = styled.div<OpenProps>`
  position: absolute;
  top: 46px;
  left: 0;
  overflow: hidden;
  background: ${({ theme }) => theme.colors.levels.popout};
  border-radius: 4px;
  z-index: 99;
  box-shadow: ${({ theme }) => theme.boxShadow[1]};
  opacity: ${p => (p.open ? 1 : 0)};
  visibility: ${p => (p.open ? 'visible' : 'hidden')};
  transform-origin: top center;
  transition: opacity 0.2s ease, visibility 0.2s ease,
    transform 0.3s cubic-bezier(0.45, 0.6, 0.5, 1.25);
  transform: translate3d(0, ${p => (p.open ? '12px' : 0)}, 0);
`;

const DropdownItem = styled.div<ActiveProps & OpenProps>`
  color: ${props => props.theme.colors.text.main};
  padding: 12px 16px;
  width: 190px;
  font-weight: ${p => (p.active ? 700 : 400)};
  box-sizing: border-box;
  cursor: pointer;
  opacity: ${p => (p.open ? 1 : 0)};
  transition: transform 0.3s ease, opacity 0.7s ease;
  transform: translate3d(0, ${p => (p.open ? 0 : '-10px')}, 0);

  &:hover,
  &:focus {
    outline: none;
    background: ${({ theme }) => theme.colors.spotBackground[0]};
  }
`;

const Arrow = styled.div<OpenProps>`
  position: absolute;
  top: 50%;
  right: 16px;
  transform: translate(0, -50%);
  color: ${props => props.theme.colors.text.main}
  line-height: 0;

  svg {
    transform: ${p => (p.open ? 'rotate(-180deg)' : 'none')};
    transition: 0.1s linear transform;

    path {
      fill: ${props => props.theme.colors.text.main}
    }
  }
`;

export function NavigationSwitcher(props: NavigationSwitcherProps) {
  const [open, setOpen] = useState(false);

  const ref = useRef<HTMLDivElement>();
  const activeValueRef = useRef<HTMLDivElement>();
  const firstValueRef = useRef<HTMLDivElement>();

  const activeItem = props.items.find(item => item.category === props.value);
  const requiresAttentionButNotActive = props.items.some(
    item => item.requiresAttention && item.category !== activeItem.category
  );

  const handleClickOutside = useCallback(
    (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as HTMLElement)) {
        setOpen(false);
      }
    },
    [ref.current]
  );

  useEffect(() => {
    if (open) {
      document.addEventListener('mousedown', handleClickOutside);

      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
      };
    }
  }, [ref, open, handleClickOutside]);

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent) => {
      switch (event.key) {
        case 'Enter':
          setOpen(open => !open);

          break;

        case 'Escape':
          setOpen(false);

          break;

        case 'ArrowDown':
          if (!open) {
            setOpen(true);
          }

          firstValueRef.current.focus();

          break;

        case 'ArrowUp':
          setOpen(false);

          break;
      }
    },
    [open]
  );

  const handleKeyDownLink = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>, item: NavigationCategory) => {
      switch (event.key) {
        case 'Enter':
          handleChange(item);

          break;

        case 'ArrowDown':
          const nextSibling = event.currentTarget.nextSibling as HTMLDivElement;
          if (nextSibling) {
            nextSibling.focus();
          }

          break;

        case 'ArrowUp':
          const previousSibling = event.currentTarget
            .previousSibling as HTMLDivElement;
          if (previousSibling) {
            previousSibling.focus();

            return;
          }

          activeValueRef.current.focus();

          break;
      }
    },
    [props.value]
  );

  const handleChange = useCallback(
    (value: NavigationCategory) => {
      if (props.value !== value) {
        props.onChange(value);
      }

      setOpen(false);
    },
    [props.value]
  );

  const items = [];

  for (const [index, item] of props.items.entries()) {
    items.push(
      <DropdownItem
        ref={index === 0 ? firstValueRef : null}
        onKeyDown={event => handleKeyDownLink(event, item.category)}
        tabIndex={open ? 0 : -1}
        onClick={() => handleChange(item.category)}
        key={index}
        open={open}
        active={item.category === props.value}
      >
        {item.category}
        {item.requiresAttention && item.category !== activeItem.category && (
          <DropDownItemAttentionDot data-testid="dd-item-attention-dot" />
        )}
      </DropdownItem>
    );
  }

  return (
    <Container ref={ref}>
      {requiresAttentionButNotActive && (
        <NavSwitcherAttentionDot data-testid="nav-switch-attention-dot" />
      )}
      <ActiveValue
        ref={activeValueRef}
        onClick={() => setOpen(!open)}
        open={open}
        tabIndex={0}
        onKeyDown={handleKeyDown}
        data-testid="nav-switch-button"
      >
        {activeItem.category}

        <Arrow open={open}>
          <ChevronDownIcon />
        </Arrow>
      </ActiveValue>

      <Dropdown open={open}>{items}</Dropdown>
    </Container>
  );
}

const NavSwitcherAttentionDot = styled.div`
  position: absolute;
  background-color: ${props => props.theme.colors.error.main};
  width: 10px;
  height: 10px;
  border-radius: 50%;
  right: -3px;
  top: -4px;
  z-index: 100;
`;

const DropDownItemAttentionDot = styled.div`
  display: inline-block;
  margin-left: 10px;
  margin-top: 2px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: ${props => props.theme.colors.error.main};
`;
