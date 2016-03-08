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
        <li key={index} className={className}>
          <IndexLink to={i.to}>
            <i className={i.icon} title={i.title}/>
          </IndexLink>
        </li>
      );
    });

    items.push((
      <li key={menuItems.length}>
        <a href={cfg.helpUrl}>
          <i className="fa fa-question" title="help"/>
        </a>
      </li>));

    return (
      <nav className='grv-nav navbar-default navbar-static-side' role='navigation'>
        <div className=''>
          <ul className='nav' id='side-menu'>
            <li><div className="grv-circle text-uppercase"><span>{getUserNameLetter()}</span></div></li>
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

function getUserNameLetter(){
  var {shortDisplayName} = reactor.evaluate(getters.user);
  return shortDisplayName;
}

module.exports = NavLeftBar;
