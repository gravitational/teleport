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

import { useState, useCallback } from "react";
import styled from 'styled-components';

import { Menu } from "shared/components/Menu";
import { HeadlessButton } from "shared/components/HeadlessButton";
import { Icon } from "shared/components/Icon";
import { Logo } from "shared/components/Logo";

import blockBodyScroll from "shared/utils/blockBodyScroll";

import { NavigationHeaderCTA } from "./NavigationHeaderCTA";

export const NavigationHeader = () => {
  const [isNavigationVisible, setIsNavigationVisible] =
    useState<boolean>(false);
  const toggleNavigaton = useCallback(() => {
    setIsNavigationVisible((value) => !value);
    blockBodyScroll(isNavigationVisible);
  }, [isNavigationVisible]);

  return (
    <HeaderWrapper>
      <LogoLink href="/">
        <Logo />
      </LogoLink>
      <StyledHeadlessButton
        onClick={toggleNavigaton}
        data-testid="hamburger"
        aria-details="Toggle Main navigation"
      >
        <Icon name={isNavigationVisible ? "close" : "hamburger"} size="md" />
      </StyledHeadlessButton>
      <ContentBlock className={isNavigationVisible ? 'visible' : ''}>
        <Menu />
        <NavigationHeaderCTA />
      </ContentBlock>
    </HeaderWrapper>
  );
};

const LogoLink = styled.a`
	box-sizing: border-box;
  display: flex;
  align-items: center;
  flex-shrink: 0;
  width: 125px;
  min-width: 125px;
  height: 24px;
  padding: 0px 8px;
  color: #512fc9;
  font-size: 18px;
  font-weight: 700;
  line-height: 80px;
  text-decoration: none;
  transition: background 300ms;
  @media (min-width: 1201px) {
    padding: 0 32px;
    width: 200px;
  }
  @media (max-width: 900px) {
    width: 150px;
    min-width: 150px;
    height: 48px;
    padding: 0 16px;
    line-height: 48px;
  }
`;

const HeaderWrapper = styled.header`
	box-sizing: border-box;
	display: flex;
	align-items: center;
	flex-basis: auto;
	height: 80px;
	left: 0;
	position: absolute;
	right: 0;
	top: 0;
	z-index: 2000;

	@media (max-width: 900px) {
		position: fixed;
		height: 48px;
		background-color: #fff;
		box-shadow: 0 1px 4px rgba(0 0 0 / 24%);
	}
`;

const StyledHeadlessButton = styled(props => <HeadlessButton {...props}/>)`
	position: absolute;
	right: 0;
	top: 0;
	height: 48px;
	padding: 12px 16px;
	color: #512fc9;
	cursor: pointer;
	outline: none;

	@media (min-width: 901px) {
    display: none;
	}
`;

const ContentBlock = styled.div`
  display: flex;
  width: 100%;
  flex-shrink: 1;
  box-sizing: border-box;
  position: initial;
  overflow: visible;
  background-color: transparent;
  padding: 0px;
  z-index: 1;
  min-width: 0;
  @media (max-width: 900px) {
    position: fixed;
    z-index: 2000;
    top: 48px;
    right: 0;
    bottom: 0;
    left: 0;
    flex-direction: column;
    display: none;
    overflow: auto;
    width: auto;
    padding: 8px;
    background-color: #fff;

    &.visible {
      display: flex;
    }
  }
`;
