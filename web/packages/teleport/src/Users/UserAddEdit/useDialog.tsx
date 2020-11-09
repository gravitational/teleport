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
import { ResetToken, User } from 'teleport/services/user';

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

  function onChangeName(name = '') {
    setName(name);
  }

  function onChangeRoles(roles = [] as Option[]) {
    setSelectedRoles(roles);
  }

  function onSave() {
    const u = {
      name,
      roles: selectedRoles.map(r => r.value),
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
    roles: props.roles,
    isNew: props.isNew,
    attempt,
    name,
    selectedRoles,
    token,
  };
}

export type Props = {
  isNew: boolean;
  user: User;
  roles: string[];
  onClose(): void;
  onCreate(user: User): Promise<any>;
  onUpdate(user: User): Promise<any>;
};
