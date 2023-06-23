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

import { useState, useCallback, useRef, MouseEvent } from "react";

import styled from 'styled-components';

import {
  DropdownMenuOverlay,
  DropdownMenuCTA,
  DropdownMenuItemCTA,
} from "shared/components/DropdownMenu";

import { Button } from "shared/components/Button";

import { useClickOutside } from "shared/hooks/useClickOutside";

export const NavigationHeaderCTA = () => {
  const ref = useRef(null);

  const [isSignInVisible, setIsSignInVisible] = useState<boolean>(false);
  const toggleSignIn = useCallback((e: MouseEvent<HTMLElement>) => {
    e.preventDefault();

    setIsSignInVisible((value) => !value);
  }, []);

  useClickOutside(ref, () => {
    if (isSignInVisible) {
      setIsSignInVisible(false);
    }
  });

  return (
    <>
      {isSignInVisible && <DropdownMenuOverlay />}
      <Wrapper>
        <GroupBlock ref={ref}>
          <Button
            as="link"
            href="https://teleport.sh/"
            onClick={toggleSignIn}
            variant="secondary"
            className="cta"
            data-testid="sign-in"
          >
            Sign In
          </Button>
          <Dropdown
            className={isSignInVisible ? 'visible' : ''}
            data-testid="sign-in-menu"
          >
            <DropdownMenuCTA title="Sign in to Teleport">
              <DropdownMenuItemCTA
                href="https://teleport.sh/"
                icon="clouds"
                title="Teleport Cloud Login"
                description="Login to your Teleport Account"
              />
              <DropdownMenuItemCTA
                href="https://dashboard.gravitational.com/web/login/"
                icon="download"
                title="Dashboard Login"
                description="Legacy Login &amp; Teleport Enterprise Downloads"
              />
            </DropdownMenuCTA>
          </Dropdown>
        </GroupBlock>
        <Button
          as="link"
          href="/pricing/"
          data-testid="get-started"
          className="cta"
        >
          Get Started
        </Button>
      </Wrapper>
    </>
  );
};

const Wrapper = styled.div`
	display: flex;
	align-items: center;
	justify-content: flex-end;
	flex-grow: 0;
	flex-shrink: 0;
	margin-left: auto;
	@media (max-width: 900px) {
		flex-direction: column;
		margin-top: 32px;
		width: 100%;
		margin-left: 0;
	}

	@media (min-width: 901px) {
		flex-direction: row;
		margin-left: auto;
	}
`;

const GroupBlock = styled.div`
	position: relative;
  @media (max-width: 900px) {
    width: 100%;
  }
`;

const Dropdown = styled.div`
	display: none;
  z-index: 3000;

  @media (max-width: 900px) {
    position: relative;
    width: 100%;
  }

  @media (min-width: 901px) {
    position: absolute;
    top: 32px;
    right: 16px;
    min-width: 400px;
  }

  &.visible {
    display: block;
  }
`;
