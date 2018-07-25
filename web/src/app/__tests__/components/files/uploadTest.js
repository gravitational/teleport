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
import { React, makeHelper, ReactDOM } from 'app/__tests__/domUtils';
import { FileUploadSelector } from 'app/components/files/upload';

const $node = $('<div>');
const helper = makeHelper($node);

describe('components/files/upload', function () {

  beforeEach(()=> {
    helper.setup()
  });

  afterEach(() => {
    helper.clean();
  })

  it('should render selected files', () => {
    const onUpload = () => {
    }

    const cmpt = render({
      onUpload
    });

    cmpt.onFileSelected({
      target: {
        files: [{name: 'file1'}, { name: 'file2'} ]
      }
    })

    let $el = $node.find('.grv-file-transfer-upload-selected-files');
    expect($el.length).toEqual(1)
    expect($el.text().trim()).toEqual('2 files selected')
  });

  it('should call onUpload', () => {
    const uploader = new FileUploadSelector({
      onUpload() { }
    });

    expect.spyOn(uploader, 'setFocus');

    let actual = [];
    uploader.state.files = [{ name: 'file1' }, { name: 'file2' }];
    uploader.state.remoteLocation = '~/';
    uploader.props.onUpload = (loc, fname) => actual.push({ loc, fname })
    uploader.onUpload();

    expect(actual.length).toEqual(2);
    expect(actual[0].loc).toEqual('~/');
    expect(actual[0].fname).toEqual('file1');
    expect(actual[1].loc).toEqual('~/');
    expect(actual[1].fname).toEqual('file2');
  });

  const render = props => {
    return ReactDOM.render((
      <FileUploadSelector {...props}
        />
    ), $node[0]);
  }

});