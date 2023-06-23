/*
Copyright 2023 Gravitational, Inc.

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

import styled from 'styled-components';

import { Link } from "shared/components/Link";

import { IconName, Icon } from "shared/components/Icon";

export interface MenuCTAItemProps {
  title: string;
  href?: string;
  description?: string;
  children?: MenuCTAItemProps[];
  icon?: IconName;
  passthrough?: boolean;
}

export const DropdownMenuItemCTA = ({
  icon,
  title,
  description,
  href,
  passthrough = true, // If no value is sent, default to true
}: MenuCTAItemProps) => {
  return (
    <LinkContainer
      href={href}
      passthrough={passthrough}
    >
      {icon && <StyledIcon name={icon} />}
      <StrongItemTitle className='itemTitle'>{title}</StrongItemTitle>
      {description && <Description className='description'>{description}</Description>}
    </LinkContainer>
  );
};

const StyledIcon = styled(props => <Icon {...props}/>)`
  float: left;
  height: 24px;
  width: 24px;
  margin-right: 8px;
  margin-top: 4px;
`;

const LinkContainer = styled(props => <Link {...props}/>)`
	display: block;
  overflow: hidden;
  line-height: 24px;
  text-align: left;
  text-decoration: none;
  font-weight: 400;
  color: #512fc9;
  border: none;
  transition: 300ms;
  padding: 8px 0;
  &:focus,
  &:hover,
  &:active {
    background-color: #f0f2f4;
  }
  & + & {
    margin-top: 8px;
  }
  @media (max-width: 900px) {
    border: 1px solid #f0f2f4;
    border-radius: 2px;
  }
`;

const StrongItemTitle = styled.strong`
  display: block;
  font-size: 16px;
  font-weight: 700;
  line-height: 32px;
`;

const Description = styled.span`
  display: block;
  font-size: 14px;
  line-height: 24px;
  color: #37474f;
  text-align: left;
`;
