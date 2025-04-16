/*
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

import { useMemo, useRef, useState } from 'react';
import styled from 'styled-components';

import { Box } from 'design';
import Popover from 'design/Popover';
import { debounce } from 'shared/utils/highbar';

export default function JoinedUsers(props) {
  const { active, users, open = false, ml, mr } = props;
  const ref = useRef(null);
  const [isOpen, setIsOpen] = useState(open);

  const handleOpen = useMemo(() => {
    return debounce(() => setIsOpen(true), 300);
  }, []);

  function onMouseEnter() {
    handleOpen.cancel();
    handleOpen();
  }

  function handleClose() {
    handleOpen.cancel();
    setIsOpen(false);
  }

  if (users.length < 2) {
    return null;
  }

  const $users = users.map((u, index) => {
    const name = u.user || '';
    const initial = name.trim().charAt(0).toUpperCase();
    return (
      <UserItem key={`${index}${u.user}`}>
        <StyledAvatar>{initial}</StyledAvatar>
        {u.user}
      </UserItem>
    );
  });

  return (
    <StyledUsers
      active={active}
      ml={ml}
      mr={mr}
      ref={ref}
      onMouseLeave={handleClose}
      onMouseEnter={onMouseEnter}
    >
      {users.length}
      <Popover
        open={isOpen}
        anchorEl={ref.current}
        onClose={handleClose}
        anchorOrigin={{
          vertical: 'top',
          horizontal: 'center',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'center',
        }}
      >
        <Box
          minWidth="200px"
          bg="levels.elevated"
          borderRadius="8px"
          onMouseLeave={handleClose}
        >
          {$users}
        </Box>
      </Popover>
    </StyledUsers>
  );
}

const StyledUsers = styled.div`
  display: flex;
  width: 16px;
  height: 16px;
  font-size: 11px;
  font-weight: bold;
  overflow: hidden;
  align-items: center;
  flex-shrink: 0;
  border-radius: 50%;
  justify-content: center;
  margin-right: 3px;
  color: ${props => props.theme.colors.text.primaryInverse};
  background-color: ${props =>
    props.active
      ? props.theme.colors.brand
      : props.theme.colors.text.slightlyMuted};
`;

const StyledAvatar = styled.div`
  background: ${props => props.theme.colors.buttons.primary.default};
  color: ${props => props.theme.colors.buttons.primary.text};
  border-radius: 50%;
  display: flex;
  justify-content: center;
  align-items: center;
  font-size: 12px;
  font-weight: bold;
  height: 24px;
  margin-right: 16px;
  width: 24px;
`;

const UserItem = styled.div`
  border-bottom: 1px solid ${props => props.theme.colors.spotBackground[1]};
  color: ${props => props.theme.colors.text.main};
  font-size: 12px;
  align-items: center;
  display: flex;
  padding: 8px;
  &:last-child {
    border: none;
  }
`;
