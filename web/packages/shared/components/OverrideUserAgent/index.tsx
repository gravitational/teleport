/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { PropsWithChildren, useEffect, useRef } from 'react';

export enum UserAgent {
  Windows = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/118.0.2088.88',
  macOS = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15',
  Linux = 'Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/119.0',
}

/**
 * OverrideUserAgent overrides window.navigator.userAgent. It's reserved for use only at the top
 * level in stories. Useful for creating stories for components which return different results based
 * on the return value of getPlatform.
 */
export const OverrideUserAgent: React.FC<
  PropsWithChildren<{ userAgent: UserAgent }>
> = ({ userAgent, children }) => {
  const originalUserAgentRef = useRef<string>(window.navigator.userAgent);

  if (window.navigator.userAgent !== userAgent) {
    // Unfortunately, we cannot do this from useEffect as it needs to happen before children are
    // rendered. Otherwise they'd render with the user agent set to the original value.
    Object.defineProperty(window.navigator, 'userAgent', {
      get: () => userAgent,
      configurable: true,
    });
  }

  useEffect(() => {
    // https://storybook.js.org/docs/configure/environment-variables#with-vite
    if (!import.meta.env.STORYBOOK) {
      throw new Error(
        'OverrideUserAgent is meant to be run only from within stories'
      );
    }

    return () => {
      Object.defineProperty(window.navigator, 'userAgent', {
        get: () => originalUserAgentRef.current,
        configurable: true,
      });
    };
  }, []);

  return <>{children}</>;
};
