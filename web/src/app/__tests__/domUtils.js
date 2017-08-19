import ReactTestUtils from 'react-addons-test-utils';
import * as ReactDOM from 'react-dom';
import expect from 'expect';
import $ from 'jQuery';

export * as React from 'react'
export { ReactDOM }

export const makeHelper = node => {
 
  const $node = $(node);

  return {

    setup() {      
      $node.appendTo("body");
    },

    clean() {            
      ReactDOM.unmountComponentAtNode($node[0]);    
      $(node).remove();      
    },

    setText(el, val) {
      ReactTestUtils.Simulate.change(el, { target: { value: val } });
    },
    
    shouldExist(selector){
      expect($node.find(selector).length).toBe(1);
    },

    shouldNotExist(selector){
      expect($node.find(selector).length).toBe(0);
    }
  }
}