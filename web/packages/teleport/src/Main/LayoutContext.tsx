/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, {
  createContext,
  PropsWithChildren,
  useContext,
  useState,
} from 'react';

interface LayoutContextValue {
  hasDockedElement: boolean;
  setHasDockedElement: (value: boolean) => void;
}

const LayoutContext = createContext<LayoutContextValue>(null);

export function LayoutContextProvider(props: PropsWithChildren<unknown>) {
  const [hasDockedElement, setHasDockedElement] = useState(false);

  return (
    <LayoutContext.Provider value={{ hasDockedElement, setHasDockedElement }}>
      {props.children}
    </LayoutContext.Provider>
  );
}

export function useLayout() {
  return useContext(LayoutContext);
}
