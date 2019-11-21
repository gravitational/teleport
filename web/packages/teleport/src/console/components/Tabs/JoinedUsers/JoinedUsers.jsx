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
import { debounce } from 'lodash';
import { Text, Card } from 'design';

export default function JoinedUsers({ users, open = false, ml, mr }) {
  const ref = React.useRef(null);
  const [isOpen, setIsOpen] = React.useState(open);

  const handleOpen = React.useMemo(() => {
    return debounce(() => setIsOpen(true), 600);
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

  const $users = users.map((u, index) => (
    <Text typography="h5" py="2" key={`${index}${u.user}`}>
      {u.user}
    </Text>
  ));

  return (
    <StyledUsers
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
        <Card
          bg="white"
          color="black"
          px={4}
          py={2}
          minWidth="200px"
          onMouseLeave={handleClose}
        >
          {$users}
        </Card>
      </Popover>
    </StyledUsers>
  );
}

const StyledUsers = styled.div`
  display: flex;
  width: 14px;
  height: 14px;
  font-size: 10px;
  overflow: hidden;
  position: relative;
  align-items: center;
  flex-shrink: 0;
  user-select: none;
  border-radius: 50%;
  justify-content: center;
  background-color: #2196f3;
`;
