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

/**
 * BaseView is a recursive type representing a view in a Wizard flow.
 *
 * @template T - Any view-specific properties.
 */
export type BaseView<T> = T & {
  /**
   * Whether to hide the view from the list of views.
   */
  hide?: boolean;
  /**
   * Current step index in the wizard.
   */
  index?: number;
  /**
   * Current visible step index in the wizard (ignoring any hidden steps).
   */
  displayIndex?: number;
  /**
   * Optional list of sub-views.
   */
  views?: BaseView<T>[];
  /**
   * Title of this view in the wizard flow.
   */
  title: string;
};

/**
 * computeViewChildrenSize calculates how many children a view has, without counting the first
 * child. This is because the first child shares the same index with its parent, so we don't
 * need to count it as it's not taking up a new index.
 *
 * If `constrainToVisible` is true, then we only count the visible views.
 */
export function computeViewChildrenSize<T>({
  views,
  constrainToVisible = false,
}: {
  views: BaseView<T>[];
  constrainToVisible?: boolean;
}) {
  let size = 0;
  for (const view of views) {
    if (constrainToVisible && view.hide) {
      continue;
    }

    if (view.views) {
      size += computeViewChildrenSize({
        views: view.views,
        constrainToVisible,
      });
    } else {
      size++;
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
  index = 0,
  displayIndex = 1
): BaseView<T>[] {
  const result: BaseView<T>[] = [];

  for (const view of views) {
    const copy = {
      ...view,
      index,
      parent,
    };

    if (view.views) {
      copy.views = addIndexToViews(view.views, index, displayIndex);
      index += computeViewChildrenSize({ views: view.views });
    } else {
      index++;
    }

    if (!view.hide) {
      copy.displayIndex = displayIndex;
      displayIndex += view.views
        ? computeViewChildrenSize({
            views: view.views,
            constrainToVisible: true,
          })
        : 1;
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
