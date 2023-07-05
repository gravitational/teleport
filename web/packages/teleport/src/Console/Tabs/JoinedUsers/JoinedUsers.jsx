/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react';
import styled from 'styled-components';
import Popover from 'design/Popover';
import theme from 'design/theme';
import { Box } from 'design';
import { debounce } from 'shared/utils/highbar';

export default function JoinedUsers(props) {
  const { active, users, open = false, ml, mr } = props;
  const ref = React.useRef(null);
  const [isOpen, setIsOpen] = React.useState(open);

  const handleOpen = React.useMemo(() => {
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
          bg="white"
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
  background-color: ${props =>
    props.active ? theme.colors.brandAccent : theme.colors.grey[900]};
`;

const StyledAvatar = styled.div`
  background: ${props => props.theme.colors.brandAccent};
  color: ${props => props.theme.colors.light};
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
  border-bottom: 1px solid ${theme.colors.grey[50]};
  color: ${theme.colors.grey[600]};
  font-size: 12px;
  align-items: center;
  display: flex;
  padding: 8px;
  &:last-child {
    border: none;
  }
`;
