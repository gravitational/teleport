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

import React, { useContext } from 'react';

import type { TeleportFeature } from 'teleport/types';

interface FeaturesContextState {
  features: TeleportFeature[];
}

interface FeaturesContextProviderProps {
  value: TeleportFeature[];
}

const FeaturesContext = React.createContext<FeaturesContextState>(null);

export function FeaturesContextProvider(
  props: React.PropsWithChildren<FeaturesContextProviderProps>
) {
  return (
    <FeaturesContext.Provider value={{ features: props.value }}>
      {props.children}
    </FeaturesContext.Provider>
  );
}

export function useFeatures() {
  const { features } = useContext(FeaturesContext);

  return features;
}
