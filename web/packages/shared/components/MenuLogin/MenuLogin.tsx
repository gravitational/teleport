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

import React, { useImperativeHandle, useRef, useState } from 'react';
import styled from 'styled-components';
import { NavLink } from 'react-router-dom';
import Menu, { MenuItem } from 'design/Menu';
import { space } from 'design/system';

import { ButtonBorder, Flex, Indicator } from 'design';
import { CarrotDown } from 'design/Icon';

import { useAsync, Attempt } from 'shared/hooks/useAsync';

import { MenuLoginProps, LoginItem, MenuLoginHandle } from './types';

export const MenuLogin = React.forwardRef<MenuLoginHandle, MenuLoginProps>(
  (props, ref) => {
    const {
      onSelect,
      anchorOrigin,
      transformOrigin,
      required = true,
      width,
    } = props;
    const anchorRef = useRef<HTMLElement>();
    const [isOpen, setIsOpen] = useState(false);
    const [getLoginItemsAttempt, runGetLoginItems] = useAsync(() =>
      Promise.resolve().then(() => props.getLoginItems())
    );

    const placeholder = props.placeholder || 'Enter login name…';
    const onOpen = () => {
      if (!getLoginItemsAttempt.status) {
        runGetLoginItems();
      }
      setIsOpen(true);
    };
    const onClose = () => {
      setIsOpen(false);
    };
    const onItemClick = (
      e: React.MouseEvent<HTMLAnchorElement>,
      login: string
    ) => {
      onClose();
      onSelect(e, login);
    };
    const onKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === 'Enter' && (!required || e.currentTarget.value)) {
        onClose();
        onSelect(e, e.currentTarget.value);
      }
    };

    useImperativeHandle(ref, () => ({
      open: () => {
        onOpen();
      },
    }));

    return (
      <React.Fragment>
        <ButtonBorder
          height="24px"
          size="small"
          setRef={anchorRef}
          onClick={onOpen}
        >
          CONNECT
          <CarrotDown ml={2} mr={-2} fontSize="2" color="text.secondary" />
        </ButtonBorder>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={anchorRef.current}
          open={isOpen}
          onClose={onClose}
          getContentAnchorEl={null}
        >
          <LoginItemList
            getLoginItemsAttempt={getLoginItemsAttempt}
            onKeyPress={onKeyPress}
            onClick={onItemClick}
            placeholder={placeholder}
            width={width}
          />
        </Menu>
      </React.Fragment>
    );
  }
);

const LoginItemList = ({
  getLoginItemsAttempt,
  onClick,
  onKeyPress,
  placeholder,
  width,
}: {
  getLoginItemsAttempt: Attempt<LoginItem[]>;
  onClick: (e: React.MouseEvent<HTMLAnchorElement>, login: string) => void;
  onKeyPress: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  placeholder: string;
  width?: string;
}) => {
  const content = getLoginItemListContent(getLoginItemsAttempt, onClick);

  return (
    <Flex flexDirection="column" width={width}>
      <Input
        p="2"
        m="2"
        onKeyPress={onKeyPress}
        type="text"
        autoFocus
        placeholder={placeholder}
        autoComplete="off"
      />
      {content}
    </Flex>
  );
};

function getLoginItemListContent(
  getLoginItemsAttempt: Attempt<LoginItem[]>,
  onClick: (e: React.MouseEvent<HTMLAnchorElement>, login: string) => void
) {
  switch (getLoginItemsAttempt.status) {
    case '':
    case 'processing':
      return (
        <Indicator
          css={({ theme }) => `
            align-self: center;
            color: ${theme.colors.brand.secondaryAccent}
          `}
        />
      );
    case 'error':
      // Ignore errors and let the caller handle them outside of this component. There's little
      // space to show the error inside the menu.
      return null;
    case 'success':
      const logins = getLoginItemsAttempt.data;

      return logins.map((item, key) => {
        const { login, url } = item;
        return (
          <StyledMenuItem
            key={key}
            px="2"
            mx="2"
            as={url ? NavLink : StyledButton}
            to={url}
            onClick={(e: React.MouseEvent<HTMLAnchorElement>) => {
              onClick(e, login);
            }}
          >
            {login}
          </StyledMenuItem>
        );
      });
  }
}

const StyledButton = styled.button`
  color: inherit;
  border: none;
  flex: 1;
`;

const StyledMenuItem = styled(MenuItem)(
  ({ theme }) => `
  color: ${theme.colors.grey[400]};
  font-size: 12px;
  border-bottom: 1px solid ${theme.colors.subtle};
  min-height: 32px;
  &:hover {
    color: ${theme.colors.link};
  }

  :last-child {
    border-bottom: none;
    margin-bottom: 8px;
  }
`
);

const Input = styled.input(
  ({ theme }) => `
  background: ${theme.colors.subtle};
  border: 1px solid ${theme.colors.subtle};
  border-radius: 4px;
  box-sizing: border-box;
  color: ${theme.colors.grey[900]};
  height: 32px;
  outline: none;

  &:focus {
    background: ${theme.colors.light};
    border 1px solid ${theme.colors.link};
    box-shadow: inset 0 1px 3px rgba(0, 0, 0, .24);
  }

  ::placeholder {
    color: ${theme.colors.grey[100]};
  }
`,
  space
);
