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
import { ButtonBorder, Text } from 'design';
import Menu, { MenuItem } from 'design/Menu';
import { ChevronDown } from 'design/Icon';

import { AwsRole } from 'shared/services/apps';

export class AwsLaunchButton extends React.Component<Props> {
  anchorEl = React.createRef();

  state = {
    open: false,
    anchorEl: null,
  };

  onOpen = () => {
    this.setState({ open: true });
  };

  onClose = () => {
    this.setState({ open: false });
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
            overflow: 'auto',
            minWidth: '180px',
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
            awsRoles={awsRoles}
            getLaunchUrl={getLaunchUrl}
            onLaunchUrl={onLaunchUrl}
            closeMenu={this.onClose}
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
  onLaunchUrl,
}: Props & { closeMenu: () => void }) {
  const awsRoleItems = awsRoles.map((item, key) => {
    const { display, arn } = item;
    const launchUrl = getLaunchUrl(arn);
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
        <Text style={{ maxWidth: '25ch' }}>{display}</Text>
      </StyledMenuItem>
    );
  });

  return (
    <>
      <Text
        px="2"
        fontSize="11px"
        mb="2"
        css={`
          color: ${props => props.theme.colors.text.main};
          background: ${props => props.theme.colors.spotBackground[2]};
        `}
      >
        Select IAM Role
      </Text>
      {awsRoleItems.length ? (
        awsRoleItems
      ) : (
        <Text px={2} m={2} color="text.disabled">
          No roles found
        </Text>
      )}
    </>
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
