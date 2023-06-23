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

import { DropdownMenuItem } from "./DropdownMenuItem";

export interface DropdownMenuProps {
  title: string;
  children: ReactNode;
  titleLink?: boolean;
  href?: string;
}

export const DropdownSubMenu = ({
  title,
  children,
  titleLink = false,
  href = "/",
}: DropdownMenuProps) => {
  return (
    <Wrapper>
      {title && titleLink ? (
        <DropdownMenuItem href={href} title={title} titleLink={true} />
      ) : (
        title && <Title>{title}</Title>
      )}
      <ChildrenBlock>{children}</ChildrenBlock>
    </Wrapper>
  );
};

const Wrapper = styled.div`
  background: white;
  color: #01172c;
  overflow: hidden;
  width: 100%;
  max-width: 330px;
  padding: 16px 16px 32px 16px;
  @media (max-width: 900px) {
    max-width: none;
    padding: 0;
  }
  @media (min-width: 1201px) {
    padding-left: 10px;
    padding-right: 10px;
  }
`;

const Title = styled.h3`
	display: block;
  margin: 0 16px;
  font-size: 16px;
  font-weight: 700;
  line-height: 48px;
  @media (max-width: 900px) {
    display: inline;
  }
`;

const ChildrenBlock = styled.div`
 	padding-bottom: 8px;
  @media (max-width: 900px) {
    padding-bottom: 16px;
  }
`;
