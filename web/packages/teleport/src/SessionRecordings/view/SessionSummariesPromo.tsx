/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  arrow,
  autoUpdate,
  FloatingArrow,
  FloatingPortal,
  offset,
  shift,
  useDismiss,
  useFloating,
  useFocus,
  useHover,
  useInteractions,
  useRole,
} from '@floating-ui/react';
import {
  useId,
  useRef,
  useState,
  type ComponentType,
  type ReactNode,
} from 'react';
import styled, { useTheme } from 'styled-components';

import { ButtonText } from 'design/Button';
import Flex from 'design/Flex';
import {
  ChevronDown,
  Clock,
  ListThin,
  Magnifier,
  ShieldWarning,
  Sparkle,
} from 'design/Icon';
import type { IconProps } from 'design/Icon/Icon';
import Text, { H3 } from 'design/Text';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import { KeysEnum } from 'teleport/services/storageService';
import {
  PROMO_POPOVER_Z_INDEX,
  SESSION_SUMMARIES_DOCS_URL,
} from 'teleport/SessionRecordings/sessionSummaries';

const COPY = {
  available: {
    title: 'Session Summaries',
    intro:
      'Teleport can process sessions like this to write searchable summaries so that you can:',
  },
  needsUpgrade: {
    title: 'Session Summaries',
    intro:
      'Teleport Identity Security can summarize sessions like this so that you can:',
  },
};

const CAPABILITIES: {
  Icon: ComponentType<IconProps>;
  label: string;
  title: string;
  detail: string;
}[] = [
  {
    Icon: ListThin,
    label: 'Read a recap',
    title: 'Read a recap of what happened',
    detail:
      'Teleport captures each command and writes it in plain language so that you can read a session instead of watching it.',
  },
  {
    Icon: ShieldWarning,
    label: 'Review the risk',
    title: 'Review the risk for each session',
    detail:
      'Teleport assigns a risk score to each session so that sessions needing a closer look can rise to the top of the list.',
  },
  {
    Icon: Clock,
    label: 'Jump to the moment',
    title: 'Jump to the moment of each event',
    detail:
      'Teleport adds markers to the timeline for notable events so that you can click to jump to each one in the recording.',
  },
  {
    Icon: Magnifier,
    label: 'Search across sessions',
    title: 'Search across sessions in plain language',
    detail:
      'Teleport has AI-assisted search so that you can use plain language to find the events and recordings you need.',
  },
];

export function SessionSummariesPromo({
  primaryAction,
  needsUpgrade = false,
}: {
  primaryAction: ReactNode;
  needsUpgrade?: boolean;
}) {
  const [collapsed, setCollapsed] = useLocalStorage(
    KeysEnum.SESSION_SUMMARIES_PROMO_COLLAPSED,
    false
  );

  const bodyId = useId();

  const { title, intro } = needsUpgrade ? COPY.needsUpgrade : COPY.available;

  return (
    <Panel data-testid="session-summaries-promo" $collapsed={collapsed}>
      <H3>
        <Disclosure
          type="button"
          aria-expanded={!collapsed}
          aria-controls={bodyId}
          onClick={() => setCollapsed(!collapsed)}
        >
          <Sparkle size="large" color="interactive.solid.primary.default" />
          <Title>{title}</Title>
          <Chevron size="small" $collapsed={collapsed} />
        </Disclosure>
      </H3>

      <Collapsible $collapsed={collapsed}>
        <Body id={bodyId} inert={collapsed}>
          <Content>
            <Text typography="body2" px={2}>
              {intro}
            </Text>

            <Timeline as="ul">
              {CAPABILITIES.map(capability => (
                <Capability key={capability.label} {...capability} />
              ))}
            </Timeline>

            <Flex flexDirection="column" gap={2} px={2}>
              {primaryAction}

              <ButtonText
                as="a"
                href={SESSION_SUMMARIES_DOCS_URL}
                target="_blank"
                size="small"
                width="100%"
                color="text.slightlyMuted"
              >
                Learn more
              </ButtonText>
            </Flex>
          </Content>
        </Body>
      </Collapsible>
    </Panel>
  );
}

// We do not use `HoverTooltip` because it can't open on keyboard focus.
function Capability({
  Icon,
  label,
  title,
  detail,
}: (typeof CAPABILITIES)[number]) {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const arrowRef = useRef<SVGSVGElement>(null);

  const { refs, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement: 'right',
    middleware: [
      offset(12),
      shift({ padding: 8 }),
      arrow({ element: arrowRef, padding: 8 }),
    ],
    whileElementsMounted: autoUpdate,
  });

  const { getReferenceProps, getFloatingProps } = useInteractions([
    useHover(context, { move: false }),
    useFocus(context),
    useDismiss(context),
    useRole(context, { role: 'tooltip' }),
  ]);

  return (
    <>
      <Row ref={refs.setReference} tabIndex={0} {...getReferenceProps()}>
        <Stop>
          <Icon size="small" />
        </Stop>
        <Text typography="body2" fontWeight="bold">
          {label}
        </Text>
      </Row>

      {open && (
        <FloatingPortal>
          <Tip
            ref={refs.setFloating}
            style={floatingStyles}
            {...getFloatingProps()}
          >
            <FloatingArrow
              ref={arrowRef}
              context={context}
              fill={theme.colors.tooltip.background}
            />

            <Text typography="h3" color="text.primaryInverse">
              {title}
            </Text>
            <Text typography="body2" color="text.primaryInverse">
              {detail}
            </Text>
          </Tip>
        </FloatingPortal>
      )}
    </>
  );
}

const COLLAPSE_TRANSITION = '200ms ease';

const Panel = styled(Flex)<{ $collapsed: boolean }>`
  flex-direction: column;
  flex-shrink: 0;
  margin: 0 ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  padding: ${p => p.theme.space[p.$collapsed ? 2 : 4]}px
    ${p => p.theme.space[2]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  overflow-y: auto;
  min-height: 0;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[2]};
  transition: padding ${COLLAPSE_TRANSITION};

  @media (prefers-reduced-motion: reduce) {
    transition: none;
  }
`;

const Disclosure = styled.button`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[3]}px;
  width: 100%;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
  border: none;
  border-radius: ${p => p.theme.radii[2]}px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;

  &:focus-visible {
    outline: 2px solid ${p => p.theme.colors.interactive.solid.accent.default};
    outline-offset: 1px;
  }
`;

const Title = styled.span`
  flex: 1;
`;

const Chevron = styled(ChevronDown)<{ $collapsed: boolean }>`
  color: ${p => p.theme.colors.text.slightlyMuted};
  transform: rotate(${p => (p.$collapsed ? 0 : 180)}deg);
  transition:
    transform ${COLLAPSE_TRANSITION},
    color ${COLLAPSE_TRANSITION};

  ${Disclosure}:hover & {
    color: ${p => p.theme.colors.text.main};
  }

  @media (prefers-reduced-motion: reduce) {
    transition: none;
  }
`;

const Collapsible = styled.div<{ $collapsed: boolean }>`
  display: grid;
  grid-template-rows: ${p => (p.$collapsed ? '0fr' : '1fr')};
  transition: grid-template-rows ${COLLAPSE_TRANSITION};

  @media (prefers-reduced-motion: reduce) {
    transition: none;
  }
`;

const Body = styled.div`
  min-height: 0;
  overflow: hidden;
`;

const Content = styled(Flex)`
  flex-direction: column;
  gap: ${p => p.theme.space[2]}px;
  padding-top: ${p => p.theme.space[3]}px;
`;

const Timeline = styled(Flex)`
  position: relative;
  flex-direction: column;
  gap: ${p => p.theme.space[2]}px;
  margin: 0;
  padding: 0 0 ${p => p.theme.space[1]}px;
  list-style: none;

  &::before {
    content: '';
    position: absolute;
    top: 18px;
    bottom: 22px;
    left: 21px;
    width: 2px;
    background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }
`;

const Row = styled(Flex).attrs({ as: 'li' })`
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
  position: relative;
  z-index: 1;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
  border-radius: ${p => p.theme.radii[2]}px;
  cursor: default;

  &:hover,
  &:focus-visible {
    background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  }

  &:focus-visible {
    outline: 2px solid ${p => p.theme.colors.interactive.solid.accent.default};
    outline-offset: 1px;
  }
`;

const Stop = styled(Flex)`
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;

const Tip = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[2]}px;
  width: 306px;
  padding: ${p => p.theme.space[3]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  background: ${p => p.theme.colors.tooltip.background};
  backdrop-filter: blur(2px);
  box-shadow: ${p => p.theme.boxShadow[0]};
  z-index: ${PROMO_POPOVER_Z_INDEX};
`;
