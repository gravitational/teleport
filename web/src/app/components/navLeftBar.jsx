var React = require('react');
var { Router, IndexLink, History } = require('react-router');
var getters = require('app/modules/user/getters');
var cfg = require('app/config');

var menuItems = [
  {icon: 'fa fa-cogs', to: cfg.routes.nodes, title: 'Nodes'},
  {icon: 'fa fa-sitemap', to: cfg.routes.sessions, title: 'Sessions'}
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
          <i className="fa fa-sign-out"></i>
        </a>
      </li>
    ));

    return (
      <nav className='grv-nav navbar-default' role='navigation'>
        <ul className='nav text-center' id='side-menu'>
          <li title="current user"><div className="grv-circle text-uppercase"><span>{getUserNameLetter()}</span></div></li>
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
