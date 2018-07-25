/*
Copyright 2018 Gravitational, Inc.

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

import expect from 'expect';
import $ from 'jQuery';
import ReactTestUtils from 'react-addons-test-utils';
import { React, makeHelper, ReactDOM } from 'app/__tests__/domUtils';
import { FileDownloadSelector } from 'app/components/files/download';

const $node = $('<div>');
const helper = makeHelper($node);

describe('components/files/download', function () {

  beforeEach(()=> {
    helper.setup()
  });

  afterEach(() => {
    helper.clean();
  })

  it('should render', () => {
    let wasCalled = false;
    const onDownload = () => {
      wasCalled = true;
    }

    const cmpt = render({
      onDownload
    });

    // try to click download on invalid path
    keyPress(cmpt.inputRef);
    expect($node.find('.grv-file-transfer-btn:disabled').length).toEqual(1);
    expect(wasCalled).toEqual(false);

    setValue(cmpt.inputRef, "/test");
    keyPress(cmpt.inputRef);
    expect($node.find('.grv-file-transfer-btn:disabled').length).toEqual(0);
    expect(wasCalled).toEqual(true);
  });

  it('should handle invalid input', () => {
    const cmpt = new FileDownloadSelector();
    expect(cmpt.isValidPath("")).toEqual(false);
    expect(cmpt.isValidPath("/")).toEqual(false);
    expect(cmpt.isValidPath("test/")).toEqual(false);
    expect(cmpt.isValidPath("test")).toEqual(true);
  })

  const render = props => {
    return ReactDOM.render((
      <FileDownloadSelector {...props}
        />
    ), $node[0]);
  }

  const setValue = (cmpt, val) => {
    ReactTestUtils.Simulate.change(cmpt, { target: { value: val } });
  }

  const keyPress = cmpt => {
    ReactTestUtils.Simulate.keyDown(cmpt, { key: 'Enter' });
  }
});