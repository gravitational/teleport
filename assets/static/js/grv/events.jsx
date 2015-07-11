'use strict';

var EventsPage = React.createClass({
    showEvent: function(e){
        if(e.schema != "teleport.session") {
            return;
        }
        this.refs.event.show(e.properties.recordid);
    },
    getInitialState: function(){
        return {entries: []};
    },
    componentDidMount: function() {
        this.reload();
        setInterval(this.reload, this.props.pollInterval);
    },
    reload: function(){
        var q = [
            {name: 'start', value: toRFC3339(new Date())},
            {name: 'limit', value: '20'},
            {name: 'order', value: '-1'},
        ];
        $.ajax({
            url: this.props.url+ "?"+ $.param(q),
            dataType: 'json',
            success: function(data) {
                this.setState({entries: data});
            }.bind(this),
            error: function(xhr, status, err) {
                console.error(this.props.url, status, err.toString());
            }.bind(this)
        });
    },
    render: function() {
        return (
            <div id="wrapper">
              <LeftNavBar current="events"/>
              <div id="page-wrapper" className="gray-bg">
                <TopNavBar/>
                <PageHeader title="Cluster Events" url="/events"/>
                <div className="wrapper wrapper-content animated fadeInRight">
                  <Box>
                    <EventsBox entries={this.state.entries} onShowEvent={this.showEvent}/>
                  </Box>
                </div>
                <PageFooter/>
              </div>
              <EventForm ref="event"/>
            </div>);
    }
});


var EventsBox = React.createClass({
    render: function() {
        if (this.props.entries.length == 0) {
            return (
                <div className="text-center m-t-lg">
                  <h1>Logged Events</h1>
                  <small>There are no events logged. Log in via SSH to watch the events.</small><br/><br/>
                </div>);
        }
        return (
            <div>
              <EventsTable {...this.props}/>
            </div>
        );
    }
});


var EventsTable = React.createClass({
    render: function() {
        var show = this.props.onShowEvent
        var keyNodes = this.props.entries.map(function (event, index) {
            return (
                <EventRow event={event} key={index} onShowEvent={show}></EventRow>
            );
        });
        return (
            <table className="table table-striped">
              <thead>
                <tr>
                  <th></th>
                  <th>Time</th>
                  <th>Event</th>
                  <th>User</th>
                  <th>Client IP</th>
                  <th>Remote IP</th>
                  <th>Data</th>
                </tr>
              </thead>
              <tbody>
                {keyNodes}
              </tbody>
            </table>
        );
    }
});


var EventRow = React.createClass({
    showEvent: function(e) {
        e.preventDefault();
        this.props.onShowEvent(this.props.event);
    },
    describe: function(event) {
        switch (event.schema) {
            case "teleport.auth.attempt":
                if (event.properties.success == "true") {
                    return {icon: "fa fa-user text-navy", text: "successfull auth"};
                }
                return {icon: "fa fa-user text-warning", text: "unsucessfull auth: " + event.properties.error};
            case "teleport.session":
                return {icon: "fa fa-tty text-navy", text: "replay session"};
            case "teleport.message":
                return {icon: "fa fa-wechat text-navy", text: event.properties.message};
            case "teleport.exec":
                return {icon: "fa fa-tty text-navy", text: event.properties.command + " " + atob(event.properties.log).substring(0, 100)};
        }
        return {icon: "fa fa-question text-navy", text: "unrecognized event: "+event.schema};
    },
    render: function() {
        var e = this.props.event;
        var d = this.describe(e);
        return (
            <tr className="key">
              <td><a href="#"><i className={d.icon}></i></a></td>
              <td>{e.time}</td>
              <td>{e.schema}</td>
              <td>{e.properties.user}</td>
              <td>{e.properties.localaddr}</td>
              <td>{e.properties.remoteaddr}</td>
              <td><a href="#" onClick={this.showEvent}>{d.text}</a></td>
            </tr>
        );
    }
});

var EventForm = React.createClass({
    show: function(rid) {
        this.iter = 0
        this.term = new Terminal({
            cols: 120,
            rows: 24,
            useStyle: true,
            screenKeys: true,
            cursorBlink: false
        });
        this.term.open(React.findDOMNode(this.refs.term));
        this.refs.modal.open();
        if(rid == "") {
            this.term.write("this session was not recorded, or recording was deleted");
            this.player = null;
        } else {
            this.player = new Player(rid, this.term);
            this.player.start();
        }
    },
    close: function() {
        if(this.player != null) {
            this.player.stop();
        }
        this.term.destroy();
        this.refs.modal.close();
    },
    render: function() {
        return (
            <BootstrapModal dialogClass="modal-lg"
                            icon="fa-list"
                            ref="modal"
                            cancel="Close"
                            onCancel={this.close}
	                        title="SSH Session Log">
              <div ref="term" style={{width: '580px', height: '400px'}} className="text-center m-t-lg"></div>
            </BootstrapModal>
        );
    }
});


React.render(
  <EventsPage url="/api/events" pollInterval={2000}/>,
  document.body
);
