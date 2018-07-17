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

  const render = props => {
    return ReactDOM.render((
      <FileUploadSelector {...props}
        />
    ), $node[0]);
  }

});