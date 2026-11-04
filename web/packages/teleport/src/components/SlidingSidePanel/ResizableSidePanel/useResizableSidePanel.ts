/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { useCallback, useState } from 'react';

type Tab = 'info' | 'terraform';

/**
 * Holds state that control the opening/closing of the side panel,
 * the currently active tab, and modifying the width of the panel
 * (making sure it doesn't go out of minWidth/maxWidth).
 */
export function useResizableSidePanel({
  defaultWidth = 500,
  minWidth = 365,
  maxWidth = 900,
  initiallyOpen = true,
  defaultTab = 'info',
}: {
  defaultWidth?: number;
  minWidth?: number;
  maxWidth?: number;
  initiallyOpen?: boolean;
  defaultTab?: Tab;
} = {}) {
  const [isOpen, setIsOpen] = useState(initiallyOpen);
  const [activeTab, setActiveTab] = useState<Tab>(defaultTab);
  const [width, setWidth] = useState(() =>
    clampedWidth(defaultWidth, minWidth, maxWidth)
  );

  const open = useCallback(() => setIsOpen(true), []);
  const close = useCallback(() => setIsOpen(false), []);
  const toggle = useCallback(() => setIsOpen(current => !current), []);

  const selectTab = useCallback((tab: Tab) => {
    setActiveTab(tab);
    setIsOpen(true);
  }, []);

  const toggleTab = useCallback(
    (tab: Tab) => {
      if (isOpen && activeTab === tab) {
        setIsOpen(false);
        return;
      }

      setActiveTab(tab);
      setIsOpen(true);
    },
    [activeTab, isOpen]
  );

  const updateWidth = useCallback(
    (newWidth: number) => {
      const updatedWidth = clampedWidth(newWidth, minWidth, maxWidth);
      setWidth(updatedWidth);
    },
    [maxWidth, minWidth]
  );

  return {
    isOpen,
    activeTab,
    width,
    open,
    close,
    toggle,
    selectTab,
    toggleTab,
    updateWidth,
  };
}

// Forces requestedWidth to not go below minWidth or above maxWidth range.
function clampedWidth(
  requestedWidth: number,
  minWidth: number,
  maxWidth: number
) {
  const boundedMin = Math.max(requestedWidth, minWidth);
  return Math.min(boundedMin, maxWidth);
}

export function getPanelTitle(activeTab: Tab) {
  switch (activeTab) {
    case 'info':
      return 'Info Guide';
    case 'terraform':
      return 'Terraform';
    default:
      activeTab satisfies never;
      return '';
  }
}
