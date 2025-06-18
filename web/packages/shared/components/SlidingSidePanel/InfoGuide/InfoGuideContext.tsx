/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import {
  createContext,
  FC,
  PropsWithChildren,
  useContext,
  useEffect,
  useMemo,
  useState,
  type JSX,
} from 'react';

import { generalInfoPanelWidth } from './const';

type InfoGuidePanelContextState = {
  infoGuideConfig: InfoGuideConfig | null;
  setInfoGuideConfig: (cfg: InfoGuideConfig) => void;
  panelWidth: number;
};

export type InfoGuideConfig = {
  /**
   * The component that contains the guide to render.
   */
  guide: JSX.Element;
  /**
   * If true, means the view where this guide will get
   * rendered already has it's own side panel. Normally
   * the parent container that renders this guide will need
   * to set a margin-right equal to the guide's panelWidth
   * to make space to render the guide (so it doesn't render
   * over existing contents), but with this flag set to true,
   * the parent container will not set any margin-right since
   * it's assumed the space will already be accounted for.
   *
   * Eg: In unified resources view (UnifiedResourcesE.tsx) in
   * enterprise version, there exists a side panel for access request
   * checkout. If a resource is checked out, the view will render a side
   * panel that already pushes contents out of the way. If the guide
   * renders, we will use the same side panel to push contents
   * out of the way. This avoids extra widths added and width flickering
   * if we try to conditionally push contents out of the way with the
   * guides parent container when both the guide and the checkout
   * is activated.
   */
  viewHasOwnSidePanel?: boolean;
  /**
   * Optional custom title for the guide panel.
   */
  title?: React.ReactNode;
  /**
   * Optional custom panel width.
   */
  panelWidth?: number;
  /**
   * Optional ID of the component rendered.
   * Useful when there are multi guides in a page, and we need to know
   * which of the guide was activated.
   *
   * eg: we have a table with rows of different guides, and we need to
   * highlight the row where the guide was activated.
   */
  id?: string;
};

const InfoGuidePanelContext = createContext<InfoGuidePanelContextState>(null);

export const InfoGuidePanelProvider: FC<
  PropsWithChildren<{ defaultPanelWidth?: number }>
> = ({ children, defaultPanelWidth = generalInfoPanelWidth }) => {
  const [infoGuideConfig, setInfoGuideConfig] =
    useState<InfoGuideConfig | null>(null);

  const providerValue = useMemo(
    () => ({
      infoGuideConfig,
      setInfoGuideConfig,
      panelWidth: infoGuideConfig?.panelWidth || defaultPanelWidth,
    }),
    [defaultPanelWidth, infoGuideConfig]
  );

  return (
    <InfoGuidePanelContext.Provider value={providerValue}>
      {children}
    </InfoGuidePanelContext.Provider>
  );
};

/**
 * hook that allows you to set the info guide element that
 * will render in the InfoGuideSidePanel component.
 *
 * To close the InfoGuideSidePanel component, set infoGuideConfig
 * state back to null.
 */
export const useInfoGuide = () => {
  const context = useContext(InfoGuidePanelContext);

  if (!context) {
    throw new Error('useInfoGuide must be used within a InfoGuidePanelContext');
  }

  const { infoGuideConfig, setInfoGuideConfig, panelWidth } = context;

  useEffect(() => {
    return () => {
      setInfoGuideConfig(null);
    };
  }, []);

  return {
    infoGuideConfig,
    setInfoGuideConfig,
    panelWidth,
  };
};
