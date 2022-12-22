/**
 * Copyright 2022 Gravitational, Inc.
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

import React from 'react';

import { useDiscover } from 'teleport/Discover/useDiscover';
import { SelectDatabaseType } from 'teleport/Discover/Database/SelectDatabaseType';

import { Database } from 'teleport/Discover/Database/resources';

import { ActionButtons, PermissionsErrorMessage } from '../Shared';

interface DatabaseResourceProps {
  disabled: boolean;
  onProceed: () => void;
}

export function DatabaseResource(props: DatabaseResourceProps) {
  const { resourceState } = useDiscover<Database>();

  let content = <SelectDatabaseType />;

  if (props.disabled) {
    content = (
      <PermissionsErrorMessage
        action="add new Databases"
        productName="Database Access"
      />
    );
  }

  return (
    <>
      {content}

      <ActionButtons
        onProceed={() => props.onProceed()}
        disableProceed={!resourceState || props.disabled}
      />
    </>
  );
}
