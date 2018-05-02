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

import React from 'react';

const DEFAULT_TITLE = 'Teleport by Gravitational';

export class DocumentTitle extends React.Component {

  componentWillReceiveProps(nextProps) {    
    if (this.getTitle() !== nextProps.title) {
      this.setTitle(nextProps.title);
    }
  }
  
  componentDidMount() {
    this.setTitle(this.props.title);
  }

  getTitle() {
    return document.title;
  }

  setTitle(title) {
    document.title = title;
  }

  render() {    
    return this.props.children;
  }

}

export const RouteDocumentTitle = props => {
  let title = DEFAULT_TITLE
  let routes = props.routes || [];
  for (let i = routes.length - 1; i > 0; i--) {
    if (routes[i].title) {
      title = routes[i].title;
      break;
    }
  }

  document.title = title;

  return props.children;
}