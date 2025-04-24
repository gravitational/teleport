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
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
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

export const InfoGuidePanelProvider: React.FC<
  PropsWithChildren<{ defaultPanelWidth?: number }>
> = ({ children, defaultPanelWidth = generalInfoPanelWidth }) => {
  const [infoGuideConfig, setInfoGuideConfig] =
    useState<InfoGuideConfig | null>(null);

  return (
    <InfoGuidePanelContext.Provider
      value={{
        infoGuideConfig,
        setInfoGuideConfig,
        panelWidth: infoGuideConfig?.panelWidth || defaultPanelWidth,
      }}
    >
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
