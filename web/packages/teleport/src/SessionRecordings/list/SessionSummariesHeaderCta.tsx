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
  autoUpdate,
  flip,
  FloatingFocusManager,
  FloatingPortal,
  offset,
  shift,
  useClick,
  useDismiss,
  useFloating,
  useInteractions,
  useRole,
} from '@floating-ui/react';
import { useState } from 'react';
import styled from 'styled-components';

import { ButtonPrimary, ButtonText } from 'design/Button';
import Flex from 'design/Flex';
import { ShieldCheck } from 'design/Icon';
import Text, { H3 } from 'design/Text';

import {
  PROMO_POPOVER_Z_INDEX,
  SESSION_SUMMARIES_DOCS_URL,
  UpgradeButton,
} from 'teleport/SessionRecordings/sessionSummaries';

export function SessionSummariesHeaderCta() {
  const [open, setOpen] = useState(false);

  const { refs, floatingStyles, context } = useFloating({
    open,
    onOpenChange: setOpen,
    placement: 'bottom-end',
    middleware: [offset(8), flip(), shift({ padding: 8 })],
    whileElementsMounted: autoUpdate,
  });

  const { getReferenceProps, getFloatingProps } = useInteractions([
    useClick(context),
    useDismiss(context),
    useRole(context),
  ]);

  return (
    <>
      <ButtonPrimary
        size="medium"
        ref={refs.setReference}
        {...getReferenceProps()}
      >
        Summarize Sessions with AI
      </ButtonPrimary>

      {open && (
        <FloatingPortal>
          <FloatingFocusManager context={context} modal={false}>
            <Popover
              ref={refs.setFloating}
              style={floatingStyles}
              {...getFloatingProps()}
            >
              <Flex alignItems="center" gap={2}>
                <ShieldCheck size="large" />
                <H3>AI Session Summaries with Identity Security</H3>
              </Flex>

              <Text typography="body2">
                Read a recap of what happened in each session, sort sessions by
                risk, jump straight to notable events, and search in plain
                language across all of your session recordings, using Teleport
                Identity Security.
              </Text>

              <Flex flexDirection="column" gap={2}>
                <UpgradeButton size="large" />

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
            </Popover>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </>
  );
}

const Popover = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[3]}px;
  width: 360px;
  box-sizing: border-box;
  padding: ${p => p.theme.space[4]}px ${p => p.theme.space[3]}px;
  border-radius: ${p => p.theme.radii[4]}px;
  background: ${p => p.theme.colors.levels.elevated};
  box-shadow: ${p => p.theme.boxShadow[1]};
  z-index: ${PROMO_POPOVER_Z_INDEX};
`;
