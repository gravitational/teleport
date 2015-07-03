'use strict';

var SessionsPage = React.createClass({
  getInitialState: function(){
      return {sessions: []};
  },
  componentDidMount: function() {
      this.reload();
      setInterval(this.reload, this.props.pollInterval);
  },
  reload: function(){
    $.ajax({
      url: this.props.url,
      dataType: 'json',
      success: function(data) {
        if(this.modalOpen == true) {
           return
        }
        this.setState({sessions: data});
      }.bind(this),
      error: function(xhr, status, err) {
        console.error(this.props.url, status, err.toString());
      }.bind(this)
    });
  },
    render: function() {
    return (
        <div id="wrapper">
          <LeftNavBar current="sessions"/>
          <div id="page-wrapper" className="gray-bg">
            <TopNavBar/>
            <PageHeader title="Sessions" url="/sessions"/>
            <div className="wrapper wrapper-content animated fadeInRight">
              <Box>
                <SessionsTable sessions={this.state.sessions}/>
              </Box>
            </div>
            <PageFooter/>
          </div>
        </div>
    );
  }
});


var SessionsTable = React.createClass({
  render: function() {
    var onConnect = this.props.onConnect
    var onDisconnect = this.props.onDisconnect
    var rows = this.props.sessions.map(function (se, index) {
      return (
        <SessionRow se={se} key={index}/>
      );
    });
    return (
<table className="table table-striped">
   <thead>
      <tr>
         <th></th>
         <th>Parties</th>
      </tr>
   </thead>
   <tbody>
      {rows}
   </tbody>
</table>);
  }
});


var SessionRow = React.createClass({
    render: function() {
        var se = this.props.se;
        var parties = this.props.se.parties.map(function (p, index) {
            return (
                    <a href={"/sessions/"+se.id}><span className="label">{p.user} &rarr; {p.server}</span></a>
            );
        });        
        return (
                <tr>
                <td><a href="/sessions"><i className="fa fa-tty text-navy"></i></a></td>
                <td>{parties}</td>
                </tr>
        );
  }
});

React.render(
  <SessionsPage url="/api/sessions" pollInterval={1000}/>,
  document.body
);
