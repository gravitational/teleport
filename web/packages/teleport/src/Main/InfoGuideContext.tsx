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

type InfoGuidePanelContextState = {
  setInfoGuideElement: (element: JSX.Element | null) => void;
  infoGuideElement: JSX.Element | null;
};

const InfoGuidePanelContext = createContext<InfoGuidePanelContextState>(null);

export const InfoGuidePanelProvider: React.FC<PropsWithChildren> = ({
  children,
}) => {
  const [infoGuideElement, setInfoGuideElement] = useState<JSX.Element | null>(
    null
  );

  return (
    <InfoGuidePanelContext.Provider
      value={{ infoGuideElement, setInfoGuideElement }}
    >
      {children}
    </InfoGuidePanelContext.Provider>
  );
};

/**
 * hook that allows you to set the info guide element that
 * will render in the InfoGuideSidePanel component.
 *
 * To close the InfoGuideSidePanel component, set infoGuideElement
 * state back to null.
 */
export const useInfoGuide = () => {
  const { infoGuideElement, setInfoGuideElement } = useContext(
    InfoGuidePanelContext
  );

  useEffect(() => {
    return () => {
      setInfoGuideElement(null);
    };
  }, []);

  return {
    setInfoGuideElement,
    infoGuideElement,
  };
};
