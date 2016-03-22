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
var React = require('react');
var reactor = require('app/reactor');
var { IndexLink } = require('react-router');
var getters = require('app/modules/user/getters');
var cfg = require('app/config');

var menuItems = [
  {icon: 'fa fa-share-alt', to: cfg.routes.nodes, title: 'Nodes'},
  {icon: 'fa  fa-group', to: cfg.routes.sessions, title: 'Sessions'}
];

var NavLeftBar = React.createClass({

  render: function(){
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
        <a href={cfg.routes.logout}>
          <i className="fa fa-sign-out" style={{marginRight: 0}}></i>
        </a>
      </li>
    ));

    return (
      <nav className='grv-nav navbar-default' role='navigation'>
        <ul className='nav text-center' id='side-menu'>
          <li title="current user">
            <div className="btn btn-primary btn-circle text-uppercase text-uppercase">
              <strong>{getUserNameLetter()}</strong>
            </div>
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

function getUserNameLetter(){
  var {shortDisplayName} = reactor.evaluate(getters.user);
  return shortDisplayName;
}

module.exports = NavLeftBar;
