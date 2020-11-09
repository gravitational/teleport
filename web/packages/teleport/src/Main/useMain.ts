/*
Copyright 2019 Gravitational, Inc.

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

import { useState } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import useTeleport from 'teleport/useTeleport';
import { Feature } from 'teleport/types';

export default function useMain(features: Feature[]) {
  const ctx = useTeleport();
  const { attempt, run } = useAttempt('processing');

  useState(() =>
    run(() => ctx.init().then(() => features.forEach(f => f.register(ctx))))
  );

  return {
    ctx,
    status: attempt.status,
    statusText: attempt.statusText,
  };
}

export type State = ReturnType<typeof useMain>;
