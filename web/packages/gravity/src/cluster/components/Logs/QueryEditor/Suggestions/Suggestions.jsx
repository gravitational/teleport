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
import ReactDOM from 'react-dom';
import styled from 'styled-components';

class Suggestions extends React.Component {

  onMouseClick = e => {
    e.preventDefault();
    const element = e.target;
    const children = element.parentNode.children;
    for (let i = 0; i < children.length; i++){
      if (children[i] === element) {
        this.props.onMouseClick(i);
      }
    }
  }

  componentDidUpdate() {
    if(!this.popupMenuRef){
      return;
    }

    const [ activeItem ] = this.popupMenuRef.getElementsByClassName('--active');
    if(activeItem){
      // scroll
      const focusedDOM = ReactDOM.findDOMNode(activeItem);
      const menuDOM = ReactDOM.findDOMNode(this.popupMenuRef);
      const focusedRect = focusedDOM.getBoundingClientRect();
      const menuRect = menuDOM.getBoundingClientRect();
      if (focusedRect.bottom > menuRect.bottom || focusedRect.top < menuRect.top) {
        menuDOM.scrollTop = (focusedDOM.offsetTop + focusedDOM.clientHeight - menuDOM.offsetHeight);
      }
    }
   }

  renderItems(){
    const { curItem, data } = this.props;
    let $items = data.map((dataItem, index) => {
      const { text, /* type */ } = dataItem;
      const itemClass = index === curItem ? '--active' : '';
      return (
        <StyledMenuItem key={index} className={itemClass} onClick={this.onMouseClick}>
          {text}
        </StyledMenuItem>
      )
    });

    if($items.length === 0){
      $items = (
        <StyledMenuItem>
          No suggestions
        </StyledMenuItem>
      )
    }

    return $items;
  }

  render(){
    const $items = this.renderItems();
    if( $items.length === 0){
      return null;
    }

    return (
      <StyledMenu ref={ e => this.popupMenuRef = e}>
        {$items}
      </StyledMenu>
    );
  }
}

const StyledMenuItem = styled.div`
  display: block;
  padding: 10px;
  clear: both;
  font-weight: normal;
  color: #333333;
  white-space: nowrap;
  cursor: pointer;

  &.--active, &:hover {
    text-decoration: none;
    color: #262626;
    background-color: #f5f5f5;
  }
`

const StyledMenu = styled.div`
  display: block;
  background-color: white;
  width: 100%;
  position: absolute;
  border: 1px solid;
  border-color: #d9d9d9 #ccc #b3b3b3;
  font-size: 12px;
  list-style: none outside none;
  padding: 0;
  text-shadow: none;
  margin-top: -1px;
  max-height: 350px;
  overflow: auto;
`

export default Suggestions;
