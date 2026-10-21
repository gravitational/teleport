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

import { useEffect, useId, useRef, useState } from 'react';
import styled from 'styled-components';

import { Danger } from 'design/Alert';
import Box from 'design/Box';
import Flex from 'design/Flex';
import * as Icon from 'design/Icon';
import { Indicator } from 'design/Indicator';
import Text, { H4 } from 'design/Text';
import { useInfiniteScroll } from 'shared/hooks';
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
import { ScopeSearchInput } from 'teleport/Scopes/ScopeSearchInput';
import { useAssignedScopes } from 'teleport/services/scopes';
import session from 'teleport/services/websession/websession';

export type ScopeSwitcherProps = {
  /** Current scope. */
  scope: string;
};

/** A drop-down control that signs out the user and switches the scope. */
export function ScopeSwitcher({ scope }: ScopeSwitcherProps) {
  const [open, setOpen] = useState(false);
  const outsideClickRef = useRefClickOutside<HTMLDivElement>({ open, setOpen });
  const dropdownRef = useRef<HTMLDivElement | null>(null);
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const dropdownID = useId();
  const [prefix, setPrefix] = useState('');

  const {
    fetchNextPage,
    data,
    isFetching,
    isSuccess,
    isError,
    error,
    hasNextPage,
    isPlaceholderData,
  } = useAssignedScopes({ prefix, retryOnMount: false });

  const { setTrigger } = useInfiniteScroll({
    fetch: async () => {
      if (hasNextPage && !isError && !isFetching) {
        await fetchNextPage();
      }
    },
  });

  // Focus the search input when the dropdown is opened. We can't use
  // autofocus, as the dropdown uses visibility style, and autofocus is a
  // mount-only property.
  useEffect(() => {
    if (open) {
      // A translation animation is played when opening the dropdown. When
      // focusing the search input, browsers would scroll the container left to
      // fully show the translated dropdown, then when the animation finishes,
      // the container gets scrolled back again and the entire menu is visibly
      // bumped to the right again. We could use the preventScroll option, but
      // we still *do* want the container to be scrolled vertically, so let's
      // remember the current scrollLeft property and restore it after focusing
      // the input.
      const dropdown = dropdownRef.current;
      const scrollLeft = dropdown?.scrollLeft ?? 0;
      searchInputRef.current?.focus();
      if (dropdown) {
        dropdown.scrollLeft = scrollLeft;
      }
    }
  }, [open]);

  function onScopeSelected(newScope: string) {
    session.switchScope(newScope);
  }

  // If an unfiltered initial query returns no results, it means the user
  // doesn't have any scopes. Don't show the scope switcher.
  if (
    isSuccess &&
    prefix == '' &&
    data?.pages.length === 1 &&
    data?.pages[0]?.scopes?.length === 0
  ) {
    return;
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
        <ScopeNameAndIcon scope={scope} iconAriaLabel="scope" />
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
          !outsideClickRef.current.contains(e.relatedTarget as Node) &&
          setOpen(false)
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
        <DropdownPassiveItem
          open={open}
          $transitionDelay={
            STARTING_TRANSITION_DELAY + INCREMENT_TRANSITION_DELAY
          }
          my={3}
        >
          <ScopeSearchInput noIcon onChange={setPrefix} ref={searchInputRef} />
        </DropdownPassiveItem>
        <DropdownPassiveItem
          open={open}
          $transitionDelay={
            STARTING_TRANSITION_DELAY + INCREMENT_TRANSITION_DELAY * 2
          }
        >
          <H4 ml={3} mb={1}>
            Scopes
          </H4>
        </DropdownPassiveItem>
        {isSuccess &&
          data.pages.flatMap(page =>
            page.scopes.map((s, i) => (
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
                  Math.min(i + 2, 20) * INCREMENT_TRANSITION_DELAY
                }
              />
            ))
          )}
        {isSuccess && data?.pages[0]?.scopes.length === 0 && (
          <DropdownPassiveItem
            open={open}
            $transitionDelay={
              STARTING_TRANSITION_DELAY + 2 * INCREMENT_TRANSITION_DELAY
            }
            textAlign="center"
            my={2}
          >
            No scopes found
          </DropdownPassiveItem>
        )}
        <div ref={setTrigger} />
        {isError && <Danger>{error.message}</Danger>}
        {!isPlaceholderData && (hasNextPage || isFetching) && (
          <Box alignSelf="center">
            <Indicator
              style={{ visibility: isFetching ? 'visible' : 'hidden' }}
            />
          </Box>
        )}
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
  overflow-y: auto;
  overflow-x: hidden;
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

/**
 * A dropdown item that doesn't contain a clickable element and doesn't have hover styles.
 */
const DropdownPassiveItem = styled(Box)<{
  open: boolean;
  $transitionDelay: number;
}>`
  opacity: ${p => (p.open ? 1 : 0)};
  transform: translate3d(${p => (p.open ? 0 : '20px')}, 0, 0);
  transition:
    transform 0.3s ease,
    opacity 0.7s ease;
  transition-delay: ${p => p.$transitionDelay}ms;
  flex-shrink: 0;
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

function ScopeNameAndIcon({
  scope,
  iconAriaLabel,
}: {
  scope: string;
  iconAriaLabel?: string;
}) {
  const ScopeIcon = scope ? Icon.Contract : Icon.Home;
  return (
    <Flex gap={2} flex={1}>
      <ScopeIcon aria-label={iconAriaLabel} />
      <Text typography="body2" whiteSpace="nowrap">
        {scope || 'Teleport Home'}
      </Text>
    </Flex>
  );
}
