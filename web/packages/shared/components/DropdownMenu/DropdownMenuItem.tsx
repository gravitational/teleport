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

export interface MenuItemProps {
  title: string;
  href?: string;
  titleLink?: boolean;
  children?: MenuItemProps[];
  passthrough?: boolean;
}

export const DropdownMenuItem = ({
  title,
  href = "/",
  titleLink = false,
}: MenuItemProps) => {
  return (
    <LinkWrapper href={href}>
      <StrongStyled className={titleLink ? 'asTitle' : ''}>
        {title}
      </StrongStyled>
    </LinkWrapper>
  );
};

const LinkWrapper = styled(props => <Link {...props}/>)`
  display: block;
  overflow: hidden;
  padding: 0.5px 16px;
  border-radius: 2px;
  line-height: 24px;
  text-align: left;
  text-decoration: none;
  color: #01172c;
  @media (max-width: 900px) {
    padding: 4px 16px;
  }

  &:focus,
  &:hover {
    color: #512fc9;
  }
  & + & {
    margin-top: 8px;
  }
`;

const StrongStyled = styled.strong`
	display: block;
	font-size: 16px;
	font-weight: 400;
	line-height: 32px;

  &.asTitle {
    font-weight: 700;
	  line-height: 48px;
  }
`;
