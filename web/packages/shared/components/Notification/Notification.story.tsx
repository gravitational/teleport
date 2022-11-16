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

import React, { useState } from 'react';
import { Info, Warning } from 'design/Icon';

import { Notification } from './Notification';

export default {
  title: 'Shared/Notification',
};

export const AutoRemovable = () => {
  const [showInfo, setShowInfo] = useState(true);
  const [showWarning, setShowWarning] = useState(true);
  const [showError, setShowError] = useState(true);
  return (
    <>
      {showInfo ? (
        <Notification
          mt={4}
          item={{
            id: crypto.randomUUID(),
            severity: 'info',
            content:
              "This will be automatically removed after 5 seconds. Click to expand it. Mouseover it to restart the timer. Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
          }}
          onRemove={() => setShowInfo(false)}
          Icon={Info}
          getColor={theme => theme.colors.info}
          isAutoRemovable={true}
          autoRemoveDurationMs={5000}
        />
      ) : (
        <div>Info notification has been removed</div>
      )}
      {showWarning ? (
        <Notification
          mt={4}
          item={{
            id: crypto.randomUUID(),
            severity: 'warn',
            content:
              "This will be automatically removed after 5 seconds. Click to expand it. Mouseover it to restart the timer. Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
          }}
          onRemove={() => setShowWarning(false)}
          Icon={Warning}
          getColor={theme => theme.colors.warning}
          isAutoRemovable={true}
          autoRemoveDurationMs={5000}
        />
      ) : (
        <div>Warning notification has been removed</div>
      )}
      {showError ? (
        <Notification
          mt={4}
          item={{
            id: crypto.randomUUID(),
            severity: 'error',
            content:
              "This can only be removed by clicking on the X. Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s.",
          }}
          onRemove={() => setShowError(false)}
          Icon={Warning}
          getColor={theme => theme.colors.danger}
          isAutoRemovable={false}
        />
      ) : (
        <div>Error notification has been removed</div>
      )}
    </>
  );
};
