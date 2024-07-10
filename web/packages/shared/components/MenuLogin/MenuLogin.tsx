/**
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

import React, { useImperativeHandle, useRef, useState } from 'react';
import styled from 'styled-components';
import { NavLink } from 'react-router-dom';
import Menu, { MenuItem } from 'design/Menu';
import { space, SpaceProps } from 'design/system';

import { ButtonBorder, Flex, Indicator } from 'design';
import { ChevronDown } from 'design/Icon';

import { useAsync, Attempt } from 'shared/hooks/useAsync';

import { MenuLoginProps, LoginItem, MenuLoginHandle } from './types';

export const MenuLogin = React.forwardRef<MenuLoginHandle, MenuLoginProps>(
  (props, ref) => {
    const {
      onSelect,
      anchorOrigin,
      transformOrigin,
      alignButtonWidthToMenu = false,
      required = true,
      width,
    } = props;
    const anchorRef = useRef<HTMLButtonElement>();
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
          width={alignButtonWidthToMenu ? width : null}
          textTransform={props.textTransform}
          size="small"
          setRef={anchorRef}
          onClick={onOpen}
        >
          Connect
          <ChevronDown ml={1} size="small" color="text.slightlyMuted" />
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
    <Flex flexDirection="column" minWidth={width}>
      <Input
        p="2"
        m="2"
        // this prevents safari from adding the autofill options which would cover the available logins and make it
        // impossible to select. "But why would it do that? this isn't a username or password field?".
        // Safari includes parsed words in the placeholder as well to determine if that autofill should show.
        // Since our placeholder has the word "login" in it, it thinks its a login form.
        // https://github.com/gravitational/teleport/pull/31600
        // https://stackoverflow.com/questions/22661977/disabling-safari-autofill-on-usernames-and-passwords
        name="notsearch_password"
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
          css={`
            align-self: center;
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
  font-family: inherit;
  color: inherit;
  border: none;
  flex: 1;
`;

const StyledMenuItem = styled(MenuItem)(
  ({ theme }) => `
  background: transparent;
  font-size: 12px;
  border-bottom: 1px solid ${theme.colors.spotBackground[0]};
  min-height: 32px;

  :last-child {
    border-bottom: none;
    margin-bottom: 8px;
  }
`
);

const Input = styled.input<SpaceProps>(
  ({ theme }) => `
  background: transparent;
  border: 1px solid ${theme.colors.text.muted};
  border-radius: 4px;
  box-sizing: border-box;
  color: ${theme.colors.text.main};
  height: 32px;
  outline: none;

  &:focus, &:hover {
    border 1px solid ${theme.colors.text.slightlyMuted};
    outline: none;
  }

  ::placeholder {
    color: ${theme.colors.text.muted};
    opacity: 1;
  }
`,
  space
);
