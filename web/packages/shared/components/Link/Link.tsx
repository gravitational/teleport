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
import NextLink, { LinkProps as NextLinkProps } from "next/link";

import { isExternalLink, isHash, isLocalAssetFile } from "shared/utils/url";

import { useNormalizedHref } from "./hooks";


export interface LinkProps extends Omit<NextLinkProps, "href"> {
  passthrough?: boolean;
  scheme?: string;
  className?: string;
  href: string;
  onClick?: () => void;
  children: ReactNode;
}

export const Link = ({
  children,
  href,
  className,
  as,
  replace,
  scroll,
  shallow,
  passthrough,
  prefetch,
  locale,
  scheme,
  ...linkProps
}: LinkProps) => {
  const normalizedHref = useNormalizedHref(href);
  if (
    passthrough ||
    isHash(normalizedHref) ||
    isLocalAssetFile(normalizedHref)
  ) {
    return (
      <StyledLink
        href={href}
        {...linkProps}
        className={`${scheme ?? ''} ${className ?? ''}`}
      >
        {children}
      </StyledLink>
    );
  }

  if (isExternalLink(normalizedHref)) {
    return (
      <StyledLink
        target="_blank"
        rel="noopener noreferrer"
        {...linkProps}
        className={`${scheme ?? ''} ${className ?? ''}`}
      >
        {children}
      </StyledLink>
    );
  }

  // At this point, we return Link from the next/link package
  const nextProps: NextLinkProps = {
    ...linkProps,
    href: normalizedHref,
    as,
    replace,
    scroll,
    shallow,
    prefetch,
    locale,
  };

  return (
    <NextLink passHref {...nextProps} legacyBehavior>
      <StyledLink className={`${scheme ?? ''} ${className ?? ''}`}>
        {children}
      </StyledLink>
    </NextLink>
  );
};

const StyledLink = styled.a`
  box-sizing: border-box;
  min-width: 0;
  transition: color 300ms;

  color: #009cf1;

  &:visited {
    color: #512fc9;
  }

  &:hover,
  &:active,
  &:focus {
    color: #651fff;
  }
`;
