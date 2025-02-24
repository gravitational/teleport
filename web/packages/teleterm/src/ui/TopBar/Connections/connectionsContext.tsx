/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
  useCallback,
  useContext,
  useState,
} from 'react';

/**
 * ConnectionsContext allows other parts of the app to control the connection list.
 */
export type ConnectionsContext = {
  isOpen: boolean;
  open: (step?: Step) => void;
  close: () => void;
  toggle: () => void;
  /**
   * stepToOpen is the step that will be shown when the connection list gets opened.
   * It doesn't control the current stop beyond the initial render.
   */
  stepToOpen: Step;
};

export type Step = 'connections' | 'vnet';

const defaultStep: Step = 'connections';

export const ConnectionsContext = createContext<ConnectionsContext>(null);

export const ConnectionsContextProvider: FC<PropsWithChildren> = props => {
  const [isOpen, setIsOpen] = useState(false);
  const [stepToOpen, setStepToOpen] = useState<Step>('connections');

  const toggle = useCallback(() => {
    setIsOpen(wasOpen => !wasOpen);

    if (isOpen) {
      setStepToOpen(defaultStep);
    }
  }, [isOpen]);

  const close = useCallback(() => {
    setIsOpen(false);
    setStepToOpen(defaultStep);
  }, []);

  const open = useCallback((step: Step = defaultStep) => {
    setIsOpen(true);
    setStepToOpen(step);
  }, []);

  return (
    <ConnectionsContext.Provider
      value={{ isOpen, toggle, close, open, stepToOpen }}
    >
      {props.children}
    </ConnectionsContext.Provider>
  );
};

export const useConnectionsContext = () => {
  const context = useContext(ConnectionsContext);

  if (!context) {
    throw new Error(
      'useConnectionsContext must be used within a ConnectionsContextProvider'
    );
  }

  return context;
};
