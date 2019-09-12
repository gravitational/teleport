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

import React, { useCallback } from 'react';
import AjaxPoller from 'gravity/components/AjaxPoller';

const POLL_INTERVAL = 10000; // 10 sec

export default function Poller({namespace, onFetch}){
  const onRefresh = useCallback( () => onFetch(namespace), [namespace]);
  return (
    <AjaxPoller
      key={`${namespace}`}
      time={POLL_INTERVAL}
      onFetch={onRefresh}
    />
  )
}

