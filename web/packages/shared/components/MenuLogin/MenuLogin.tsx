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

import React, {
  ChangeEvent,
  useImperativeHandle,
  useRef,
  useState,
} from 'react';
import { NavLink } from 'react-router-dom';
import styled from 'styled-components';

import { ButtonBorder, Flex, Indicator, Text } from 'design';
import { ChevronDown } from 'design/Icon';
import Menu, { MenuItem } from 'design/Menu';
import { space, SpaceProps } from 'design/system';
import { Attempt, useAsync } from 'shared/hooks/useAsync';

import {
  LoginItem,
  MenuInputType,
  MenuLoginHandle,
  MenuLoginProps,
} from './types';

export const MenuLogin = React.forwardRef<MenuLoginHandle, MenuLoginProps>(
  (props, ref) => {
    const {
      onSelect,
      anchorOrigin,
      transformOrigin,
      alignButtonWidthToMenu = false,
      inputType = MenuInputType.INPUT,
      required = true,
      width,
      style,
    } = props;
    const [filter, setFilter] = useState('');
    const anchorRef = useRef<HTMLButtonElement>(null);
    const [isOpen, setIsOpen] = useState(false);
    const [getLoginItemsAttempt, runGetLoginItems] = useAsync(() =>
      Promise.resolve().then(() => props.getLoginItems())
    );

    const logins = getLoginItemsAttempt?.data || [];
    const filteredLogins =
      getLoginItemsAttempt?.data?.filter(item =>
        item.login.toLocaleLowerCase().includes(filter)
      ) || [];

    const defaultPlaceholder =
      inputType === MenuInputType.INPUT
        ? 'Enter login name…'
        : 'Search logins…';
    const placeholder = props.placeholder || defaultPlaceholder;

    const onOpen = () => {
      if (!getLoginItemsAttempt.status) {
        runGetLoginItems();
      }
      setIsOpen(true);
    };

    const onClose = () => {
      setFilter('');
      setIsOpen(false);
    };

    const onItemClick = (
      e: React.MouseEvent<HTMLAnchorElement>,
      login: string
    ) => {
      onClose();
      onSelect(e, login);
    };

    const onChange = (event: ChangeEvent<HTMLInputElement>) => {
      setFilter(event.target.value);
    };

    const onKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key !== 'Enter') {
        return;
      }
      // if we are a filter type input, send in the first filtered item
      // into onSelect
      if (inputType === MenuInputType.FILTER) {
        const firstFilteredItem = filteredLogins[0];
        if (!firstFilteredItem) {
          return;
        }
        onClose();
        onSelect(e, firstFilteredItem.login);
        return;
      }

      // otherwise, send in the currently typed value
      if (!required || e.currentTarget.value) {
        onClose();
        onSelect(e, e.currentTarget.value);
      }
    };

    useImperativeHandle(ref, () => ({
      open: () => {
        onOpen();
      },
    }));

    const ButtonComponent = props.ButtonComponent || ButtonBorder;

    return (
      <React.Fragment>
        <ButtonComponent
          width={alignButtonWidthToMenu ? width : null}
          textTransform={props.textTransform}
          size="small"
          setRef={anchorRef}
          onClick={onOpen}
          style={style}
        >
          {props.buttonText || 'Connect'}
          <ChevronDown ml={1} size="small" color="text.slightlyMuted" />
        </ButtonComponent>
        <Menu
          anchorOrigin={anchorOrigin}
          transformOrigin={transformOrigin}
          anchorEl={anchorRef.current}
          open={isOpen}
          onClose={onClose}
          getContentAnchorEl={null}
          // The list of logins is updated asynchronously, so Popover inside Menu needs to account
          // for LoginItemList changing in size.
          updatePositionOnChildResize
        >
          <LoginItemList
            getLoginItemsAttempt={getLoginItemsAttempt}
            items={inputType === MenuInputType.INPUT ? logins : filteredLogins}
            onKeyPress={onKeyPress}
            onChange={onChange}
            onClick={onItemClick}
            placeholder={placeholder}
            width={width}
            inputType={inputType}
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
  onChange,
  items,
  placeholder,
  width,
  inputType,
}: {
  getLoginItemsAttempt: Attempt<LoginItem[]>;
  items: LoginItem[];
  onClick: (e: React.MouseEvent<HTMLAnchorElement>, login: string) => void;
  onKeyPress: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  placeholder: string;
  width?: string;
  inputType?: MenuInputType;
}) => {
  const content = getLoginItemListContent(items, getLoginItemsAttempt, onClick);

  return (
    <Flex flexDirection="column" minWidth={width}>
      {inputType === MenuInputType.NONE ? (
        /* css and margin value matched with AWS Launch button <RoleItemList> */
        <Text
          px="2"
          mb={2}
          typography="body3"
          color="text.main"
          backgroundColor="spotBackground.2"
        >
          {placeholder}
        </Text>
      ) : (
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
          onChange={onChange}
          type="text"
          autoFocus
          placeholder={placeholder}
          autoComplete="off"
        />
      )}
      {content}
    </Flex>
  );
};

function getLoginItemListContent(
  items: LoginItem[],
  getLoginItemsAttempt: Attempt<LoginItem[]>,
  onClick: (e: React.MouseEvent<HTMLAnchorElement>, login: string) => void
) {
  switch (getLoginItemsAttempt.status) {
    case '':
    case 'processing':
      return (
        <Indicator
          // Without this margin, <Indicator> would cause a scroll bar to pop up and hide repeatedly.
          m={1}
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
      return items.map((item, key) => {
        const { login, url, isExternalUrl } = item;
        if (isExternalUrl) {
          return (
            <StyledMenuItem
              key={key}
              as="a"
              px="2"
              mx="2"
              href={url}
              target="_blank"
              title={login ? login : url}
              onClick={(e: React.MouseEvent<HTMLAnchorElement>) => {
                onClick(e, url);
              }}
            >
              {login ? login : url}
            </StyledMenuItem>
          );
        }
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

  /* displays ellipsis for longer string value */
  display: inline-block;
  text-align: left;
  max-width: 450px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;

  &:last-child {
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

  &::placeholder {
    color: ${theme.colors.text.muted};
    opacity: 1;
  }
`,
  space
);
