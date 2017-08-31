import React from 'react';
import classnames from 'classnames';

export const Danger = props => (  
  <div className={classnames("grv-alert grv-alert-danger", props.className)}>{props.children}</div>
)
  
