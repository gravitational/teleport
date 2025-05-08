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

import { Fragment, ReactElement, useEffect, useState } from 'react';

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
