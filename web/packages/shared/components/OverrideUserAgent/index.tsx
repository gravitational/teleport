/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useEffect, useRef } from 'react';

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
export const OverrideUserAgent: React.FC<{ userAgent: UserAgent }> = ({
  userAgent,
  children,
}) => {
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
    if (!process.env.STORYBOOK) {
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
