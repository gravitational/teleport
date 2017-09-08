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
const logoSvg = require('assets/img/svg/grv-tlpt-logo-full.svg');
const closeSvg = require('assets/img/svg/grv-icon-close.svg');

const TeleportLogo = () => (
  <svg className="grv-icon-logo-tlpt"><use xlinkHref={logoSvg}/></svg>
)

const CloseIcon = () => (
  <svg className="grv-icon-close"><use xlinkHref={closeSvg}/></svg>
)

const UserIcon = ({name='', isDark})=>{
  const iconClass = classnames('grv-icon-user', {
    '--dark' : isDark
  });

  return (
    <div title={name} className={iconClass}>
      <span>
        <strong>{name[0]}</strong>
      </span>
    </div>
  )
};

export {TeleportLogo, UserIcon, CloseIcon}
