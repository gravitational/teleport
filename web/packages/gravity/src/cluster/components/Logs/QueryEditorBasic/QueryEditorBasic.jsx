/*
Copyright 2019 Gravitational, Inc.

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
import { Input } from 'design';
import { KeyEnum } from './../QueryEditor/enums';

export default class QueryEditorBasic extends React.Component {

  setRef = e => {
    this.refInput = e;
  }

  onKeyDown = e => {
    if( e.which !== KeyEnum.RETURN) {
      return
    }

    if(this.refInput.value === this.props.query){
      return;
    }

    this.props.onChange(this.refInput.value);
  }

  setCursor(query) {
    const length = query.length;
    this.refInput.selectionEnd = length;
    this.refInput.selectionStart = length;
  }

  componentDidMount() {
    this.setCursor(this.props.query);
  }

  render() {
    const { query } = this.props;
    return (
      <Input ref={this.setRef} autoFocus
        placeholder="Search..."
        defaultValue={query}
        onKeyDown={this.onKeyDown}
      />
    )
  }
}
