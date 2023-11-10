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

import React, { useState, useEffect, Fragment, ReactElement } from 'react';

const DISPLAY_SKELETON_AFTER_MS = 150;

export function LoadingSkeleton(props: {
  count: number;
  /* Single skeleton item. */
  Element: ReactElement;
}) {
  const [canDisplay, setCanDisplay] = useState(false);

  useEffect(() => {
    const displayTimeout = setTimeout(() => {
      setCanDisplay(true);
    }, DISPLAY_SKELETON_AFTER_MS);
    return () => {
      clearTimeout(displayTimeout);
    };
  }, []);

  if (!canDisplay) {
    return null;
  }

  return (
    <>
      {new Array(props.count).fill(undefined).map((_, i) => (
        // Using index as key here is ok because these elements never change order
        <Fragment key={i}>{props.Element}</Fragment>
      ))}
    </>
  );
}
