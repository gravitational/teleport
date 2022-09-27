/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useContext } from 'react';

import { Feature } from 'teleport/types';
import { getOSSFeatures } from 'teleport/features';

interface FeaturesContextState {
  features: Feature[];
}

interface FeaturesContextProviderProps {
  value?: Feature[];
}

const FeaturesContext = React.createContext<FeaturesContextState>(null);

export function FeaturesContextProvider(
  props: React.PropsWithChildren<FeaturesContextProviderProps>
) {
  return (
    <FeaturesContext.Provider
      value={{ features: props.value || getOSSFeatures() }}
    >
      {props.children}
    </FeaturesContext.Provider>
  );
}

export function useFeatures() {
  const { features } = useContext(FeaturesContext);

  return features;
}
