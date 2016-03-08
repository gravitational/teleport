var React = require('react');
var {actions} = require('app/modules/activeTerminal/');

const SessionLeftPanel = () => (
  <div className="grv-terminal-participans">
    <ul className="nav">
      {/*
      <li><button className="btn btn-primary btn-circle" type="button"> <strong>A</strong></button></li>
      <li><button className="btn btn-primary btn-circle" type="button"> B </button></li>
      <li><button className="btn btn-primary btn-circle" type="button"> C </button></li>
      */}
      <li>
        <button onClick={actions.close} className="btn btn-danger btn-circle" type="button">
          <i className="fa fa-times"></i>
        </button>
      </li>
    </ul>
  </div>);

module.exports = SessionLeftPanel;
