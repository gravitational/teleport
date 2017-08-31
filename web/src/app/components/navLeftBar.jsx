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
import reactor from 'app/reactor';
import cfg from 'app/config';
import userGetters from 'app/flux/user/getters';
import * as AppStore from 'app/flux/app/appStore';
import { logout } from 'app/flux/user/actions';
import { IndexLink } from 'react-router';
import { UserIcon } from './icons.jsx';

export default function NavLeftBar(props) {    
  const items = AppStore.getStore().getNavItems()
  const name = reactor.evaluate(userGetters.userName);
  const $items = items.map((i, index)=>{
    var className = props.router.isActive(i.to) ? 'active' : '';
    return (
      <li key={index} className={className} title={i.title}>
        <IndexLink to={i.to}>
          <i className={i.icon} />
        </IndexLink>
      </li>
    );
  });

  $items.push((
    <li key={$items.length} title="help">
      <a href={cfg.helpUrl} target="_blank">
        <i className="fa fa-question" />
      </a>
    </li>));

  $items.push((
    <li key={$items.length} title="logout">
      <a href="#" onClick={logout} >
        <i className="fa fa-sign-out" style={{marginRight: 0}}></i>
      </a>
    </li>
  ));

  return (
    <nav className='grv-nav navbar-default' role='navigation'>
      <ul className='nav text-center' id='side-menu'>
        <li>
          <UserIcon name={name} />
        </li>
        {$items}
      </ul>
    </nav>
  );
}

NavLeftBar.propTypes = {
  router: React.PropTypes.object.isRequired
}
