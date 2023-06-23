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

import { ReactNode } from "react";
import styled from 'styled-components';

export interface DropdownMenuProps {
  title: string;
  children: ReactNode;
  displayAsRow?: boolean;
}

export const DropdownMenu = ({ title, children, displayAsRow }: DropdownMenuProps) => {
  return (
    <Wrapper
      className={displayAsRow && 'asRow'}
      data-testid="mobile-dropdown"
    >
      {title && <MenuTitle>{title}</MenuTitle>}
      <BodyBlock className={displayAsRow && "withSubMenus"}>
        {children}
      </BodyBlock>
    </Wrapper>
  );
};

const Wrapper = styled.div`
	box-sizing: border-box;
  overflow: hidden;
  width: max-content;
  padding-left: 24px;
  padding-right: 64px;
  padding-top: 16px;
  color: black;
  background: white;
  border-top: 1px solid #b3b3b3;
  max-width: 100%;
  box-shadow: 0px 4px 25px rgba(0, 0, 0, 0.15);

  @media (max-width: 900px) {
    width: 100%;
    text-align: left;
    border-top: none;
    border-bottom: 1px solid #d2dbdf;
    padding: 0;
    box-shadow: none;
  }

  &.asRow {
    width: 100vw;
    padding-right: 100px;
    padding-left: 0;
    padding-top: 0;
    @media (min-width: 1201px) {
      padding-left: 5%;
    }
  }
`;

const MenuTitle = styled.h3`
	@media (max-width: 900px) {
		display: none;
	}
	@media (min-width: 901px) {
		display: block;
		align-items: left;
		margin: 0 24px;
		border-bottom: none;
		border-color: #f0f2f4;
		font-size: 16px;
		line-height: 48px;
	}
`;

const BodyBlock = styled.div`
	box-sizing: border-box;
  display: block;
  padding: 0 0 16px;

  &.withSubMenus {
    display: flex;
    @media (max-width: 900px) {
      flex-direction: column;
    }
  }
`;
