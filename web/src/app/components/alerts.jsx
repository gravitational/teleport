/*
Copyright 2015 Gravitational, Inc.

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
import classnames from 'classnames';

export const Danger = props => (  
  <div className={classnames("grv-alert grv-alert-danger", props.className)}>{props.children}</div>
)

export const Info = props => (  
  <div className={classnames("grv-alert grv-alert-info", props.className)}>{props.children}</div>
)
  
export const Success = props => (  
  <div className={classnames("grv-alert grv-alert-success", props.className)}> <i className="fa fa-check m-r-xs" /> {props.children}</div>
)

