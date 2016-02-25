var React = require('react');
var { Router, IndexLink, History } = require('react-router');
var cfg = require('app/config');

var menuItems = [
  {icon: 'fa fa fa-sitemap', to: cfg.routes.nodes, title: 'Nodes'},
  {icon: 'fa fa-hdd-o', to: cfg.routes.sessions, title: 'Sessions'}
];

var NavLeftBar = React.createClass({

  render: function(){
    var items = menuItems.map((i, index)=>{
      var className = this.context.router.isActive(i.to) ? 'active' : '';
      return (
        <li key={index} className={className}>
          <IndexLink to={i.to}>
            <i className={i.icon} title={i.title}/>
          </IndexLink>
        </li>
      );
    });

    return (
      <nav className='' role='navigation' style={{width: '60px', float: 'left', position: 'absolute'}}>
        <div className=''>
          <ul className='nav 1metismenu' id='side-menu'>
            {items}
          </ul>
        </div>
      </nav>
    );
  }
});

NavLeftBar.contextTypes = {
  router: React.PropTypes.object.isRequired
}

module.exports = NavLeftBar;
