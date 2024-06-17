/**
 * Copyright 2020 Gravitational, Inc.
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

import { useState } from 'react';
import { useAttemptNext } from 'shared/hooks';
import { Option } from 'shared/components/Select';

import { ResetToken, User, AllUserTraits } from 'teleport/services/user';

import type { TraitsOption } from './TraitsEditor';

export default function useUserDialog(props: Props) {
  const { attempt, setAttempt } = useAttemptNext('');
  const [name, setName] = useState(props.user.name);
  const [token, setToken] = useState<ResetToken>(null);
  const [selectedRoles, setSelectedRoles] = useState<Option[]>(
    props.user.roles.map(r => ({
      value: r,
      label: r,
    }))
  );
  const [configuredTraits, setConfiguredTraits] = useState<TraitsOption[]>(() =>
    traitsToTraitsOption(props.user.allTraits)
  );

  function onChangeName(name = '') {
    setName(name);
  }

  function onChangeRoles(roles = [] as Option[]) {
    setSelectedRoles(roles);
  }

  function onSave() {
    const traitsToSave = {};
    for (const traitKV of configuredTraits) {
      traitsToSave[traitKV.traitKey.value] = traitKV.traitValues.map(
        t => t.value
      );
    }
    const u = {
      name,
      roles: selectedRoles.map(r => r.value),
      allTraits: traitsToSave,
    };

    const handleError = (err: Error) =>
      setAttempt({ status: 'failed', statusText: err.message });

    setAttempt({ status: 'processing' });
    if (props.isNew) {
      props
        .onCreate(u)
        .then(token => {
          setToken(token);
          setAttempt({ status: 'success' });
        })
        .catch(handleError);
    } else {
      props
        .onUpdate(u)
        .then(() => {
          props.onClose();
        })
        .catch(handleError);
    }
  }

  return {
    onClose: props.onClose,
    onSave,
    onChangeName,
    onChangeRoles,
    fetchRoles: props.fetchRoles,
    setConfiguredTraits,
    isNew: props.isNew,
    attempt,
    name,
    selectedRoles,
    token,
    configuredTraits,
  };
}

export type Props = {
  isNew: boolean;
  user: User;
  fetchRoles(search: string): Promise<string[]>;
  onClose(): void;
  onCreate(user: User): Promise<any>;
  onUpdate(user: User): Promise<any>;
};

export function traitsToTraitsOption(allTraits: AllUserTraits): TraitsOption[] {
  const newTrait = [];
  for (let trait in allTraits) {
    if (!allTraits[trait]) {
      continue;
    }
    if (allTraits[trait].length === 1 && !allTraits[trait][0]) {
      continue;
    }
    if (allTraits[trait].length > 0) {
      newTrait.push({
        traitKey: { value: trait, label: trait },
        traitValues: allTraits[trait].map(t => ({
          value: t,
          label: t,
        })),
      });
    }
  }
  return newTrait;
}
