import React from 'react';
import cfg from 'app/config';

export const DocsTrustedCluster = props => ( 
  <a href={cfg.trustedClusterDocLink} target="_blank"> {props.children} </a>          
)
