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
'use strict';

var UserBarItem = React.createClass({
    render: function() {
        return (
<li className="nav-header">
    <div className="dropdown profile-element">
        <a data-toggle="dropdown" className="dropdown-toggle" href="#">
            <span className="clear">
                <span className="block m-t-xs">
                    <strong className="font-bold">David Williams</strong>
                </span>
            <span className="text-muted text-xs block">
                Engineer <b className="caret"></b></span>
            </span>
       </a>
       <ul className="dropdown-menu animated fadeInRight m-t-xs">
            <li><a href="/web/logout">Logout</a></li>
       </ul>
    </div>
    <div className="logo-element">
        Gravitational Teleport
    </div>
</li>
);
    }
});

var LeftNavBar = React.createClass({
    className: function(item){
        if (item == this.props.current) {
            return "active";
        }
        return "";
    },
    render: function(){
        return (
<nav className="navbar-default navbar-static-side" role="navigation">
    <div className="sidebar-collapse">
        <ul className="nav" id="side-menu">
            <UserBarItem/>
            <li className={this.className("sites")}>
                <a href="/web/sites">
                    <i className="fa fa-cloud"></i>
                    <span className="nav-label">Sites</span>
                </a>
            </li>
        </ul>
    </div>
</nav>
);
  }
});

var LeftSiteNavBar = React.createClass({
    className: function(item){
        if (item == this.props.current) {
            return "active";
        }
        return "";
    },
    render: function(){
        return (
<nav className="navbar-default navbar-static-side" role="navigation">
    <div className="sidebar-collapse">
        <ul className="nav" id="side-menu">
            <UserBarItem/>
            <li className={this.className("sites")}>
                <a href="/web/sites">
                    <i className="fa fa-laptop"></i>
                    <span className="nav-label">Sites</span>
                    <span className="fa arrow"></span>
                </a>
                <ul className="nav nav-second-level">
                <li className="active">
                  <a href={"/web/sites/"+site.name}><span className="nav-label">{site.name}</span> <span className="fa arrow"></span></a>
                  <ul className="nav nav-third-level">
                    <li><a href={"/web/sites/"+site.name+"/servers"}>Servers</a></li>
                    <li><a href={"/web/sites/"+site.name+"/events"}>Events</a></li>
                  </ul>
                </li>
              </ul>
            </li>
        </ul>
    </div>
</nav>
);
  }
});


var TopNavBar = React.createClass({
    render: function(){
        return (
<div className="row border-bottom">
   <nav className="navbar navbar-static-top" role="navigation" style={{marginBottom: 0}}>
      <div className="navbar-header">
         <a className="navbar-minimalize minimalize-styl-2 btn btn-primary " href="#"><i className="fa fa-bars"></i></a>
         <form role="search" className="navbar-form-custom" method="post" action="search_results.html">
            <div className="form-group">
               <input placeholder="Search for something..." className="form-control" name="top-search" id="top-search" type="text"/>
            </div>
         </form>
      </div>
      <ul className="nav navbar-top-links navbar-right">
         <li>
            <span className="m-r-sm text-muted welcome-message">Welcome to Gravitational Teleport!</span>
         </li>
         <li>
            <a href="/web/logout"><i className="fa fa-sign-out"></i> Log out</a>
         </li>
      </ul>
   </nav>
</div>);
   }
    
});

var PageHeader = React.createClass({
    render: function() {
        var brs = this.props.breadcrumbs;
        return (
<div className="row wrapper border-bottom white-bg page-heading">
   <div className="col-lg-10">
      <h2><i className={this.props.icon}></i> {this.props.title}</h2>
      <ol className="breadcrumb">
        {brs.map(function(b, idx) {
          if (idx == brs.length -1 ) {
            return <li className="active"><a href={b.url}><strong>{b.title}</strong></a></li>;
          }
          return <li><a href={b.url}>{b.title}</a></li>;
        })}
      </ol>
   </div>
   <div className="col-lg-2">
   </div>
</div>);
    }
});

var PageFooter = React.createClass({
    render: function() {
        return (
<div className="footer">
    <div className="pull-right">
        You are logged in into <strong>gravitational.io</strong> teleport
    </div>
    <div>
        <strong>Copyright</strong> Gravitational &copy; 2015
    </div>
</div>);
    }
});


var Box = React.createClass({
    render: function() {
        return (
<div className="row">
   <div className="col-lg-12">
      <div className="ibox float-e-margins">
         <div className="ibox-title">
            <h5>{this.props.title}</h5>
            <div className="ibox-tools">
               <a className="collapse-link fa fa-chevron-up" style={{fontStyle:"italic"}}></a> 
               <a className="dropdown-toggle fa fa-wrench" data-toggle="dropdown" href="#" style={{fontStyle:"italic"}}></a>
               <ul className="dropdown-menu dropdown-user">
                  <li>
                     <a href="#">Config option 1</a>
                  </li>
               </ul>
               <a className="close-link fa fa-times" style={{fontStyle:"italic"}}></a>
            </div>
         </div>
         <div className="ibox-content">
            <div className="table-responsive">
                {this.props.children}
            </div>
         </div>
      </div>
   </div>
</div>
);
    }
});
