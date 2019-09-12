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

import React from 'react';
import  * as LabelStates from 'design/LabelState';
import { StatusEnum } from 'gravity/services/clusters';

export default function ClusterStatus({value}){
  let text = 'Unkown';
  let LabelState = LabelStates.StateSuccess;
  if (value === StatusEnum.READY){
    text = 'Healthy';
    LabelState = LabelStates.StateSuccess;
  } else if(value === StatusEnum.ERROR){
    text = 'With Issues';
    LabelState = LabelStates.StateDanger;
  }else if(value === StatusEnum.PROCESSING){
    text = 'In progress'
    LabelState = LabelStates.StateWarning;
  }

  return (
    <LabelState shadow>
      {text}
    </LabelState>
  )
}