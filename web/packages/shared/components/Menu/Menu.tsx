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

import { useState } from "react";
import styled from 'styled-components';

import { MenuCategory } from "./Category";
import structure from "./structure";

export const Menu = () => {
  const [openedCategoryId, setOpenedCategoryId] = useState<number | null>(null);
  return (
    <NavItems>
      {structure.map((props, id) => (
        <MenuCategory
          key={id}
          id={id}
          opened={id === openedCategoryId}
          onToggleOpened={setOpenedCategoryId}
          {...props}
        />
      ))}
    </NavItems>
  );
};

const NavItems = styled.nav`
  box-sizing: border-box;
  display: flex;
  flex-direction: row;
  width: auto;
  flex-shrink: 1;
  margin-right: 10px;
  min-width: 0;
  @media (max-width: 900px) {
    flex-direction: column;
    width: 100%;
    margin-right: 0;
    flex-shrink: 0;
    max-width: none;
  }
`;

