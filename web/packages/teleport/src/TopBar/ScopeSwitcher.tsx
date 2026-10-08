/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useId, useRef, useState } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import * as Icon from 'design/Icon';
import Text, { H4 } from 'design/Text';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import {
  commonDropdownItemStyles,
  Dropdown,
  DropdownArrow,
  DropdownButton,
  DropdownContainer,
  DropdownDivider,
  DropdownItem,
  INCREMENT_TRANSITION_DELAY,
  STARTING_TRANSITION_DELAY,
} from 'teleport/components/Dropdown';
import { focusOutsideTarget } from 'teleport/lib/util/eventTarget';
import session from 'teleport/services/websession/websession';

/** A drop-down control that signs out the user and switches the scope. */
export function ScopeSwitcher({
  scopes,
  scope,
}: {
  /** A list of available scopes. */
  scopes: string[];
  /** Current scope. */
  scope: string;
}) {
  const [open, setOpen] = useState(false);
  const outsideClickRef = useRefClickOutside<HTMLDivElement>({ open, setOpen });
  const dropdownRef = useRef<HTMLDivElement>(null);
  const dropdownID = useId();

  function onScopeSelected(newScope: string) {
    session.switchScope(newScope);
  }

  return (
    <DropdownContainer ref={outsideClickRef}>
      <DropdownButton
        as="button"
        onClick={() => setOpen(!open)}
        onBlur={e =>
          focusOutsideTarget(e, dropdownRef.current) && setOpen(false)
        }
        tabIndex={0}
        role="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={dropdownID}
      >
        <ScopeNameAndIcon scope={scope} />
        <DropdownArrow open={open}>
          <Icon.ChevronDown size="medium" />
        </DropdownArrow>
      </DropdownButton>
      <SwitcherDropdown
        open={open}
        ref={dropdownRef}
        role="menu"
        id={dropdownID}
        onBlur={e =>
          !e.currentTarget.contains(e.relatedTarget as Node) && setOpen(false)
        }
      >
        <SwitcherDropdownItem
          scope=""
          open={open}
          signedIn={scope === ''}
          onScopeSelected={onScopeSelected}
          transitionDelay={STARTING_TRANSITION_DELAY}
        />
        <DropdownDivider />
        <DropdownHeading
          ml={3}
          mt={2}
          open={open}
          $transitionDelay={
            STARTING_TRANSITION_DELAY + INCREMENT_TRANSITION_DELAY
          }
        >
          Scopes
        </DropdownHeading>
        {scopes.map((s, i) => (
          <SwitcherDropdownItem
            key={s}
            scope={s}
            open={open}
            signedIn={scope === s}
            onScopeSelected={onScopeSelected}
            transitionDelay={
              // The maximum delay is capped to account for users with a big
              // number of scopes.
              STARTING_TRANSITION_DELAY +
              Math.min(i + 1, 20) * INCREMENT_TRANSITION_DELAY
            }
          />
        ))}
      </SwitcherDropdown>
    </DropdownContainer>
  );
}

const SwitcherDropdown = styled(Dropdown)`
  left: 0;
  right: auto;
  width: auto;
  min-width: 290px;
  transform-origin: top left;
  max-height: 80vh;
  overflow-y: scroll;
`;

function SwitcherDropdownItem({
  scope,
  open,
  signedIn,
  transitionDelay,
  onScopeSelected,
}: {
  scope: string;
  open: boolean;
  signedIn: boolean;
  transitionDelay: number;
  onScopeSelected: (scope: string) => void;
}) {
  return (
    <StyledDropdownItem
      as="button"
      open={open}
      role="menuitem"
      $transitionDelay={transitionDelay}
      signedIn={signedIn}
      onClick={() => onScopeSelected(scope)}
      tabIndex={0}
      disabled={signedIn}
    >
      <ScopeNameAndIcon scope={scope} />
      {signedIn && (
        <Text typography="body2" whiteSpace="nowrap">
          Signed in
        </Text>
      )}
    </StyledDropdownItem>
  );
}

const DropdownHeading = styled(H4)<{ open: boolean; $transitionDelay: number }>`
  opacity: ${p => (p.open ? 1 : 0)};
  transform: translate3d(${p => (p.open ? 0 : '20px')}, 0, 0);
  transition:
    transform 0.3s ease,
    opacity 0.7s ease;
  transition-delay: ${p => p.$transitionDelay}ms;
`;

const StyledDropdownItem = styled(DropdownItem)<{ signedIn: boolean }>`
  ${commonDropdownItemStyles}
  display: flex;
  gap: ${props => props.theme.space[2]}px;
  ${props =>
    props.signedIn
      ? `background-color: ${props.theme.colors.interactive.tonal.neutral[2]}`
      : ''}
`;

function ScopeNameAndIcon({ scope }: { scope: string }) {
  const ScopeIcon = scope ? Icon.Contract : Icon.Home;
  return (
    <Flex gap={2} flex={1}>
      <ScopeIcon aria-label="scope" />
      <Text typography="body2">{scope || 'Teleport Home'}</Text>
    </Flex>
  );
}
