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

import React from 'react';
import styled from 'styled-components';
import { space } from 'design/system';
import { ButtonBorder, Flex, Text, Box } from 'design';
import Menu, { MenuItem } from 'design/Menu';
import { ChevronDown } from 'design/Icon';

import { AwsRole } from 'shared/services/apps';

export class AwsLaunchButton extends React.Component<Props> {
  anchorEl = React.createRef();

  state = {
    open: false,
    anchorEl: null,
    filtered: '',
  };

  onOpen = () => {
    this.setState({ open: true, filtered: '' });
  };

  onClose = () => {
    this.setState({ open: false, filtered: '' });
  };

  onChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    this.setState({ filtered: event.target.value });
  };

  render() {
    const { open } = this.state;
    const { awsRoles, getLaunchUrl, onLaunchUrl } = this.props;
    return (
      <>
        <ButtonBorder
          textTransform="none"
          width="90px"
          size="small"
          setRef={e => (this.anchorEl = e)}
          onClick={this.onOpen}
        >
          Launch
          <ChevronDown ml={1} mr={-2} size="small" color="text.slightlyMuted" />
        </ButtonBorder>
        <Menu
          menuListCss={() => ({
            overflow: 'hidden',
            minWidth: '180px',
            maxHeight: '400px',
          })}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'right',
          }}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'right',
          }}
          getContentAnchorEl={null}
          anchorEl={this.anchorEl}
          open={open}
          onClose={this.onClose}
        >
          <RoleItemList
            awsRoles={awsRoles.filter(role => {
              const lowerFilter = this.state.filtered.toLowerCase();
              const lowerDisplay = role.display.toLowerCase();
              const lowerName = role.name.toLowerCase();
              return (
                lowerDisplay.includes(lowerFilter) ||
                lowerName.includes(lowerFilter)
              );
            })}
            getLaunchUrl={getLaunchUrl}
            onLaunchUrl={onLaunchUrl}
            closeMenu={this.onClose}
            onChange={this.onChange}
          />
        </Menu>
      </>
    );
  }
}

function RoleItemList({
  awsRoles,
  getLaunchUrl,
  closeMenu,
  onChange,
  onLaunchUrl,
}: Props & {
  closeMenu: () => void;
  onChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
}) {
  const awsRoleItems = awsRoles.map((item, key) => {
    const { display, arn, name, accountId } = item;
    const launchUrl = getLaunchUrl(arn);
    let text = `${accountId}: ${display}`;
    if (display !== name) {
      text = `${text} (${name})`;
    }
    return (
      <StyledMenuItem
        as="a"
        key={key}
        px={2}
        mx={2}
        href={launchUrl}
        target="_blank"
        title={display}
        onClick={() => {
          closeMenu();
          onLaunchUrl?.(item.arn);
        }}
      >
        <Text>{text}</Text>
      </StyledMenuItem>
    );
  });

  return (
    <Flex flexDirection="column">
      <Text
        px="2"
        fontSize="11px"
        css={`
          color: ${props => props.theme.colors.text.main};
          background: ${props => props.theme.colors.spotBackground[2]};
        `}
      >
        Select IAM Role
      </Text>
      <StyledInput
        p="2"
        m="2"
        type="text"
        onChange={onChange}
        autoFocus
        placeholder={'Search IAM roles...'}
        autoComplete="off"
      />
      <Box
        css={`
          max-height: 220px;
          overflow: auto;
        `}
      >
        {awsRoleItems.length ? (
          awsRoleItems
        ) : (
          <Text px={2} m={2} color="text.disabled">
            No roles found
          </Text>
        )}
      </Box>
    </Flex>
  );
}

type Props = {
  awsRoles: AwsRole[];
  getLaunchUrl(arn: string): string;
  onLaunchUrl?(arn: string): void;
};

const StyledMenuItem = styled(MenuItem)(
  ({ theme }) => `
  font-size: 12px;
  border-bottom: 1px solid ${theme.colors.spotBackground[0]};
  min-height: 32px;
  &:hover {
    background: ${theme.colors.spotBackground[0]};
    color: ${theme.colors.text.main};
  }

  :last-child {
    border-bottom: none;
    margin-bottom: 8px;
  }
`
);

const StyledInput = styled.input(
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
