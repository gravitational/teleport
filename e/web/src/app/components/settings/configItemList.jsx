import React from 'react';
import classnames from 'classnames';

class ConfigItemList extends React.Component {
        
  renderItem(item) {
    const { onItemClick, curItem } = this.props;    
    const className = classnames('grv-settings-res-list-content-item', {
      'active': item.key === curItem.key
    });

    const displayName = item.displayName || item.name;

    return (
      <li key={item.key} className={className} onClick={() => onItemClick(item)}>
        <a>
          <span> {displayName} </span>
        </a>
      </li>
    )
  }

  render() {          
    const { onNew, items, btnText } = this.props;    
    const $items = items.map(r => this.renderItem(r));    
    return (      
      <div className="grv-settings-res-list">                                
        <div className="grv-settings-res-list-body" >          
          <ul className="grv-settings-res-list-content">
            {$items}
          </ul>                              
        </div>                                     
        <button onClick={onNew} className="grv-settings-res-new btn btn-sm m-t btn-default"> 
          <i className="fa fa-plus m-r-xs"/>
          {btnText} 
        </button>                                                                                  
      </div>
    );
  }
}


export default ConfigItemList;