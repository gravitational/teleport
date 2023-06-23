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

import { useRef, useCallback } from "react";
import styled from 'styled-components';

import { useClickOutside } from "shared/hooks/useClickOutside";

import {
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuOverlay,
  DropdownSubMenu,
  MenuItemProps,
} from "../DropdownMenu";


export interface MenuCategoryProps {
  title: string;
  description?: string;
  href?: string;
  children?: MenuItemProps[] | MenuCategoryProps[];
  containsSubCategories?: boolean;
  testId: string;
  titleLink?: boolean;
  onClick?: () => void | undefined | Promise<void>;
}

interface MenuCategoryComponentProps extends MenuCategoryProps {
  id: number;
  opened: boolean;
  onToggleOpened: (id: number | null) => void;
}

export const MenuCategory = ({
  id,
  opened,
  title,
  description,
  children,
  href,
  containsSubCategories,
  onToggleOpened,
  testId,
  onClick,
}: MenuCategoryComponentProps) => {
  const ref = useRef(null);
  const menuTestId = `${testId}-menu`;

  useClickOutside(ref, () => {
    if (opened) {
      onToggleOpened(null);
    }
  });

  const toggleOpened = useCallback(
    (e: React.MouseEvent<HTMLAnchorElement>) => {
      if (children) {
        e.preventDefault();

        onToggleOpened(opened ? null : id);
      } else {
        onClick && onClick();
      }
    },
    [opened, children, id, onToggleOpened, onClick]
  );

  return (
    <>
      {opened && <DropdownMenuOverlay />}
      <Wrapper
        className={containsSubCategories && 'withSubMenus'}
        ref={ref}
      >
        <LinkWrapper
          href={href}
          onClick={toggleOpened}
          className={opened ? 'active' : ''}
          data-testid={testId}
        >
          {title}
        </LinkWrapper>
        <DropdownWrapper
          className={`${opened ? 'opened' : ''} ${containsSubCategories ? 'withSubCategories' : ''}`}
          data-testid={menuTestId}
        >
          {children && (
            <DropdownMenu
              title={description}
              displayAsRow={containsSubCategories ? true : false}
            >
              {children.map((props) =>
                containsSubCategories ? (
                  <DropdownSubMenu
                    key={`drdwn${props.title}`}
                    title={props.title}
                    titleLink={props.titleLink}
                    href={props.href}
                  >
                    {props.children?.map((childProps) => (
                      <DropdownMenuItem
                        key={`drdwnchild${childProps.title}`}
                        {...childProps}
                      />
                    ))}
                  </DropdownSubMenu>
                ) : (
                  <DropdownMenuItem key={props.title} {...props} />
                )
              )}
            </DropdownMenu>
          )}
        </DropdownWrapper>
      </Wrapper>
    </>
  );
};

const Wrapper = styled.div`
  box-sizing: border-box;
  min-width: 0;
  margin: 0;
  padding: 0;
  position: relative;
  &.withSubMenus {
    position: initial;
  }
`;

const LinkWrapper = styled.a`
  position: relative;
  box-sizing: border-box;
  display: flex;
  flex-direction: row;
  flex-shrink: 1;
  align-items: center;
  justify-content: center;
  z-index: 3001;
  width: 100%;
  min-width: 0;
  height: 80px;
  padding: 0 16px;
  font-size: 15px;
  font-weight: 400;
  color: #37474f;
  outline: none;
  text-decoration: none;
  text-align: left;
  cursor: pointer;
  transition: background 300ms;
  margin-bottom: 0;
  float: none;
  &:focus,
  &:hover {
    color: #512fc9;
  }
  @media (max-width: 900px) {
    width: 100%;
    border-radius: 4px;
    font-size: 16px;
    line-height: 56px;
    color: #37474f;
    text-align: left;
    border-bottom: 1px solid #d2dbdf;
    justify-content: left;
  }

  &.active {
    color: #512fc9;
    @media (max-width: 900px) {
      margin-bottom: 16px;
      border-bottom: none;
    }
    background-color: #f3f5f6;
  }
`;

const DropdownWrapper = styled.div`
  display: none;
  left: 0;
  z-index: 3000;
  margin-left: 0;

  @media (max-width: 900px) {
    position: relative;
    width: 100%;
  }

  @media (min-width: 901px) {
    position: absolute;
    min-width: 540px;
    max-width: 100%;
  }

  &.opened {
    display: block;
  }
  
  &.withSubCategories {
    @media (min-width: 901px) {
      position: absolute;
    }
  }
`;

