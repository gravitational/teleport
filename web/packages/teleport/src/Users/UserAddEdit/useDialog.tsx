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

import { useState } from 'react';

import { Option } from 'shared/components/Select';
import { useAttemptNext } from 'shared/hooks';

import { ResetToken, User } from 'teleport/services/user';

import { traitsToTraitsOption, type TraitsOption } from 'shared/components/TraitsEditor';

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
