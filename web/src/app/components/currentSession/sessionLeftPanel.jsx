var React = require('react');
var {actions} = require('app/modules/activeTerminal/');
var colors = ['#1ab394', '#1c84c6', '#23c6c8', '#f8ac59', '#ED5565', '#c2c2c2'];

const UserIcon = ({name, colorIndex=0})=>{
  let color = colors[colorIndex % colors.length];
  let style = {
    'backgroundColor': color,
    'borderColor': color
  };

  return (
    <li title={name}>
      <span style={style} className="btn btn-primary btn-circle text-uppercase">
        <strong>{name[0]}</strong>
      </span>
    </li>
  )
};

const SessionLeftPanel = ({parties}) => {
  parties = parties || [];
  let userIcons = parties.map((item, index)=>(
    <UserIcon key={index} colorIndex={index} name={item.user}/>
  ));

  return (
    <div className="grv-terminal-participans">
      <ul className="nav">
        {userIcons}
        <li>
          <button onClick={actions.close} className="btn btn-danger btn-circle" type="button">
            <i className="fa fa-times"></i>
          </button>
        </li>
      </ul>
    </div>
  )
};

module.exports = SessionLeftPanel;
