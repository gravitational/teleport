import React, { PropTypes } from 'react';
import classnames from 'classnames';
import { NewButton } from './elements';

class ConfigItemList extends React.Component {
 
  static propTypes = {
    btnText: PropTypes.string.isRequired,  
    onNew: PropTypes.func.isRequired,  
    onItemClick: PropTypes.func.isRequired,  
    items: PropTypes.object.isRequired,
    curItem: PropTypes.object.isRequired
  }

  renderItem(item) {
    const { onItemClick, curItem } = this.props;    
    const className = classnames('grv-settings-res-list-content-item', {
      'active': item.id === curItem.id
    });

    const displayName = item.displayName || item.name;

    return (
      <li key={item.id} className={className} onClick={() => onItemClick(item)}>
        <a>
          <span> {displayName} </span>
        </a>
      </li>
    )
  }

  render() {          
    const { onNew, items, canCreate=true, btnText } = this.props;            
    const $items = items.map(r => this.renderItem(r));    
    return (      
      <div className="grv-settings-res-list m-r">                                
        <div className="grv-settings-res-list-body" >          
          <ul className="grv-settings-res-list-content">
            {$items}
          </ul>                              
        </div>    
        <NewButton enabled={canCreate} onClick={onNew} text={btnText} />        
      </div>
    );
  }
}


export default ConfigItemList;