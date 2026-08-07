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

import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import styled from 'styled-components';

import Flex from 'design/Flex/Flex';
import { LockKey } from 'design/Icon/Icons/LockKey';
import Text from 'design/Text/Text';
import { fontWeights } from 'design/theme/typography';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

import { LockResourceKind } from 'teleport/LocksV2/NewLock/common';

import { useResourceLock } from './useResourceLock';

/**
 * `ResourceLockIndicator` is a component to show the lock status of a resource.
 * It uses `useResourceLock` internally. If not locked, this component renders
 * nothing.
 *
 * @param props
 * @returns
 */
export function ResourceLockIndicator(props: {
  /** The kind of resource to display. */
  targetKind: LockResourceKind;
  /** The name of the resource to display. */
  targetName: string;
}) {
  const { targetKind, targetName } = props;

  const { locks } = useResourceLock({
    targetKind,
    targetName,
  });

  if (!locks || locks.length === 0) {
    return undefined;
  }

  const [{ expires, message }] = locks;

  return (
    <HoverTooltip
      placement="bottom"
      tipContent={
        locks.length > 1 ? (
          <>
            Multiple locks in-force. See{' '}
            <b>Identity Governance &gt; Session &amp; Identity Locks</b> for
            more details.
          </>
        ) : (
          <>
            {message ? `Message: ${message}` : 'Message: none'}
            <br />
            {expires
              ? `Expires: ${format(parseISO(expires), 'PP, p z')}`
              : 'Expires: never'}
          </>
        )
      }
    >
      <LockedIndicatorContainer>
        <LockKey size="medium" />
        <Text typography="body2" fontWeight={fontWeights.bold}>
          Locked
        </Text>
      </LockedIndicatorContainer>
    </HoverTooltip>
  );
}

const LockedIndicatorContainer = styled(Flex)`
  height: 32px;
  align-items: center;
  gap: ${({ theme }) => theme.space[2]}px;
  padding-left: ${({ theme }) => theme.space[3]}px;
  padding-right: ${({ theme }) => theme.space[3]}px;
  background-color: ${({ theme }) => theme.colors.interactive.tonal.danger[0]};
  border: 1px solid ${({ theme }) => theme.colors.interactive.tonal.danger[2]};
  border-radius: 16px;
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
`;
