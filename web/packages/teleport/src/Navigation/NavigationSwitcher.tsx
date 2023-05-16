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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { ChevronDownIcon } from 'design/SVGIcon/ChevronDown';

import { useLocalStorage } from 'shared/hooks/useLocalStorage';
import { OpenAIIcon } from 'design/SVGIcon/OpenAI';

import { useHistory, useLocation } from 'react-router';

import { NavigationCategory } from 'teleport/Navigation/categories';

import { useTeleport } from 'teleport';

import { KeysEnum } from 'teleport/services/localStorage';

import cfg from 'teleport/config';

import {
  TeleportIcon,
  Tooltip,
  TooltipButton,
  TooltipFooter,
  TooltipLogos,
  TooltipLogosSpacer,
  TooltipTitle,
  TooltipTitleBackground,
} from './AssistTooltip';

interface NavigationSwitcherProps {
  onChange: (value: NavigationCategory) => void;
  value: NavigationCategory;
  items: NavigationCategory[];
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
`;

const ActiveValue = styled.div<OpenProps>`
  border: 1px solid #cccccc;
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
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.45);
  opacity: ${p => (p.open ? 1 : 0)};
  visibility: ${p => (p.open ? 'visible' : 'hidden')};
  transform-origin: top center;
  transition: opacity 0.2s ease, visibility 0.2s ease,
    transform 0.3s cubic-bezier(0.45, 0.6, 0.5, 1.25);
  transform: translate3d(0, ${p => (p.open ? '12px' : 0)}, 0);
`;

const DropdownItem = styled.div<ActiveProps & OpenProps>`
  color: white;
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
    background: ${({ theme }) => theme.colors.levels.popoutHighlighted};
  }
`;

const Arrow = styled.div<OpenProps>`
  position: absolute;
  top: 50%;
  right: 16px;
  transform: translate(0, -50%);
  color: white;
  line-height: 0;

  svg {
    transform: ${p => (p.open ? 'rotate(-180deg)' : 'none')};
    transition: 0.1s linear transform;

    path {
      fill: white;
    }
  }
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

export function NavigationSwitcher(props: NavigationSwitcherProps) {
  const ctx = useTeleport();
  const assistEnabled = ctx.getFeatureFlags().assist && ctx.assistEnabled;

  const location = useLocation();
  const isAssistRoute = location.pathname.startsWith(cfg.routes.assistBase);

  const [showAssist, setShowAssist] = useLocalStorage(
    KeysEnum.SHOW_ASSIST_POPUP,
    assistEnabled && !isAssistRoute
  );

  const [open, setOpen] = useState(showAssist);

  const history = useHistory();

  const ref = useRef<HTMLDivElement>();
  const activeValueRef = useRef<HTMLDivElement>();
  const firstValueRef = useRef<HTMLDivElement>();

  const activeItem = props.items.find(item => item === props.value);

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
      setShowAssist(false);

      if (props.value !== value) {
        props.onChange(value);
      }

      if (value === NavigationCategory.Assist) {
        history.push(cfg.routes.assistBase);
      }

      setOpen(false);
    },
    [props.value]
  );

  const items = [];

  for (const [index, item] of props.items.entries()) {
    if (item === NavigationCategory.Assist && !assistEnabled) {
      continue;
    }

    items.push(
      <DropdownItem
        ref={index === 0 ? firstValueRef : null}
        onKeyDown={event => handleKeyDownLink(event, item)}
        tabIndex={open ? 0 : -1}
        onClick={() => handleChange(item)}
        key={index}
        open={open}
        active={item === props.value}
      >
        {item}
      </DropdownItem>
    );
  }

  return (
    <Container ref={ref}>
      {showAssist && (
        <>
          <Background />
          <Tooltip>
            <TooltipTitle>
              <TooltipTitleBackground>New!</TooltipTitleBackground>
            </TooltipTitle>{' '}
            Try out Teleport Assist, a GPT-4-powered AI assistant that leverages
            your infrastructure
            <TooltipFooter>
              <TooltipLogos>
                <OpenAIIcon size={30} />
                <TooltipLogosSpacer>+</TooltipLogosSpacer>
                <TeleportIcon />
              </TooltipLogos>

              <TooltipButton onClick={() => setShowAssist(false)}>
                Close
              </TooltipButton>
            </TooltipFooter>
          </Tooltip>
        </>
      )}

      <ActiveValue
        ref={activeValueRef}
        onClick={() => setOpen(!open)}
        open={open}
        tabIndex={0}
        onKeyDown={handleKeyDown}
      >
        {activeItem}

        <Arrow open={open}>
          <ChevronDownIcon />
        </Arrow>
      </ActiveValue>

      <Dropdown open={open}>{items}</Dropdown>
    </Container>
  );
}
