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
  MutableRefObject,
  PropsWithChildren,
  ReactNode,
  ReactPortal,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';

type InfoGuidePanelContextState = {
  sidePanelRef: MutableRefObject<HTMLDivElement>;
  isOpen: boolean;
  open: () => void;
  close: () => void;
  toggle: () => void;
};

const InfoGuidePanelContext = createContext<InfoGuidePanelContextState>(null);

export const InfoGuidePanelProvider: React.FC<PropsWithChildren> = ({
  children,
}) => {
  // sidePanelRef should be accessible only by the callsites that render the side panel itself, not
  // by callsites that want to render something in the side panel.
  const sidePanelRef = useRef<HTMLDivElement>();

  const [isOpen, setIsOpen] = useState(false);
  const close = useCallback(() => setIsOpen(false), []);
  const open = useCallback(() => setIsOpen(true), []);
  const toggle = useCallback(() => setIsOpen(current => !current), []);

  return (
    <InfoGuidePanelContext.Provider
      value={{ sidePanelRef, isOpen, close, open, toggle }}
    >
      {children}
    </InfoGuidePanelContext.Provider>
  );
};

// TODO: Update comment, explain which callsites this hook is for.
/**
 * hook that allows you to set the info guide element that
 * will render in the InfoGuideSidePanel component.
 *
 * To close the InfoGuideSidePanel component, set infoGuideElement
 * state back to null.
 */
export const useInfoGuide = (): Pick<
  InfoGuidePanelContextState,
  'isOpen' | 'close' | 'open' | 'toggle'
> & { createInfoGuidePortal: (node: ReactNode) => ReactPortal } => {
  const { isOpen, close, open, toggle, sidePanelRef } = useContext(
    InfoGuidePanelContext
  );

  useEffect(() => {
    return () => {
      close();
    };
  }, [close]);

  const createInfoGuidePortal = useCallback(
    (node: ReactNode) =>
      sidePanelRef.current &&
      isOpen &&
      createPortal(node, sidePanelRef.current),
    [sidePanelRef, isOpen]
  );

  return {
    isOpen,
    close,
    open,
    toggle,
    createInfoGuidePortal,
  };
};

// TODO: Explain which callsites this hook is for.
export const useSidePanel = () => {
  const { sidePanelRef, isOpen, close } = useContext(InfoGuidePanelContext);

  return { ref: sidePanelRef, isOpen, close };
};
