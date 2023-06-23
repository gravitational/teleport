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

export interface DropdownMenuCTAProps {
  title: string;
  children: ReactNode;
}

export const DropdownMenuCTA = ({ title, children }: DropdownMenuCTAProps) => {
  return (
    <Container>
      {title && <Title>{title}</Title>}
      <ChildrenBlock>{children}</ChildrenBlock>
    </Container>
  );
};

const Container = styled.div`
	box-shadow: 0 4px 40px rgba(0, 0, 0, 0.24);
  background: white;
  color: black;
  overflow: hidden;
  width: auto;
  padding-bottom: 8px;
  @media (max-width: 900px) {
    border-bottom: 1px solid #d2dbdf;
    box-shadow: none;
    width: 100%;
    padding-bottom: 0;
  }
`;

const Title = styled.h3`
	display: block;
  align-items: left;
  margin: 0 40px;
  border-bottom: 1px solid #f0f2f4;
  font-size: 18px;
  line-height: 64px;
  @media (max-width: 900px) {
    display: none;
  }
`;

const ChildrenBlock = styled.div`
	display: block;
  padding: 8px 24px 24px;
  @media (max-width: 900px) {
    padding: 8px 16px 16px;
  }
`;
