/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

export type BaseView<T> = T & {
  hide?: boolean;
  index?: number;
  views?: BaseView<T>[];
  title: string;
};

/**
 * computeViewChildrenSize calculates how many children a view has, without counting the first
 * child. This is because the first child shares the same index with its parent, so we don't
 * need to count it as it's not taking up a new index
 */
export function computeViewChildrenSize<T>(views: BaseView<T>[]) {
  let size = 0;
  for (const view of views) {
    if (view.views) {
      size += computeViewChildrenSize(view.views);
    } else {
      size += 1;
    }
  }

  return size;
}

/**
 * addIndexToViews will recursively loop over the given views, adding an index value to each one
 * The first child shares its index with the parent, as we effectively ignore the fact the parent
 * exists when trying to find the active view by the current step index.
 */
export function addIndexToViews<T>(
  views: BaseView<T>[],
  index = 0
): BaseView<T>[] {
  const result: BaseView<T>[] = [];

  for (const view of views) {
    const copy = {
      ...view,
      index,
      parent,
    };

    if (view.views) {
      copy.views = addIndexToViews(view.views, index);

      index += computeViewChildrenSize(view.views);
    } else {
      index += 1;
    }

    result.push(copy);
  }

  return result;
}

/**
 * findViewAtIndex will recursively loop views and their children in order to find the deepest
 * match at that index.
 */
export function findViewAtIndex<T>(
  views: BaseView<T>[],
  currentStep: number
): BaseView<T> | undefined {
  for (const view of views) {
    if (view.views) {
      const result = findViewAtIndex(view.views, currentStep);

      if (result) {
        return result;
      }
    }

    if (currentStep === view.index) {
      return view;
    }
  }
}
