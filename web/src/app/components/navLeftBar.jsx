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
import userGetters from 'app/modules/user/getters';
import { IndexLink } from 'react-router';
import { logoutUser } from 'app/modules/app/actions';
import { UserIcon } from './icons.jsx';

const menuItems = [
  {icon: 'fa fa-share-alt', to: cfg.routes.nodes, title: 'Nodes'},
  {icon: 'fa  fa-group', to: cfg.routes.sessions, title: 'Sessions'}
];

const NavLeftBar = React.createClass({
  render(){
    var {name} = reactor.evaluate(userGetters.user);
    var items = menuItems.map((i, index)=>{
      var className = this.context.router.isActive(i.to) ? 'active' : '';
      return (
        <li key={index} className={className} title={i.title}>
          <IndexLink to={i.to}>
            <i className={i.icon} />
          </IndexLink>
        </li>
      );
    });

    items.push((
      <li key={items.length} title="help">
        <a href={cfg.helpUrl} target="_blank">
          <i className="fa fa-question" />
        </a>
      </li>));

    items.push((
      <li key={items.length} title="logout">
        <a href="#" onClick={logoutUser} >
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
          {items}
        </ul>
      </nav>
    );
  }
});

NavLeftBar.contextTypes = {
  router: React.PropTypes.object.isRequired
}

export default NavLeftBar;
