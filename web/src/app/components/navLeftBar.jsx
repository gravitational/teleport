var React = require('react');
var Router = require('react-router');
var IndexLink = Router.IndexLink;
var History = require('react-router').History;

var menuItems = [  
  {icon: "fa fa fa-sitemap", to: `/web/nodes`, title: "Nodes"},
  {icon: "fa fa-hdd-o", to: `/web/sessions`, title: "Sessions"}
];

var NavLeftBar = React.createClass({

  mixins: [ History ],

  render: function(){
    var self = this;
    var items = menuItems.map(function(i, index){
      var className = self.history.isActive(i.to) ? "active" : "";
      return (
        <li key={index} className={className}>
          <IndexLink to={i.to}>
            <i className={i.icon} title={i.title}/>
          </IndexLink>
        </li>
      );
    });

    return (
      <nav className="" role="navigation" style={{width: '60px', float: 'left'}}>
        <div className="">
          <ul className="nav 1metismenu" id="side-menu">
            {items}
          </ul>
        </div>
      </nav>
    );
  }
});

module.exports = NavLeftBar;
