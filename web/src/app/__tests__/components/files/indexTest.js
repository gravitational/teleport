import expect from 'expect';
import $ from 'jQuery';
import ReactTestUtils from 'react-addons-test-utils';
import { React, makeHelper, ReactDOM } from 'app/__tests__/domUtils';
import { FileTransferDialog } from 'app/components/files/index';
import { FileTransferStore } from 'app/flux/fileTransfer/store';

const $node = $('<div>');
const helper = makeHelper($node);

describe('components/files', function () {

  beforeEach(()=> {
    helper.setup()
  });

  afterEach(() => {
    helper.clean();
  })

  it('should render upload controls', () => {
    let wasClosed = false
    let store = new FileTransferStore()

    store = store.merge({
      isOpen: true,
      isUpload: true
    });

    const onClose = () => {
      wasClosed = true;
    }

    const onTransfer = () => { }

    render({
      store,
      onClose,
      onTransfer
    });

    expect($node.find('.grv-file-transfer-upload').length).toEqual(1)
    expect($node.find('.grv-file-transfer-btn:disabled').length).toEqual(1)

    // verify close actions
    ReactTestUtils.Simulate.click($node.find('.grv-file-transfer-footer button')[0]);
    expect(wasClosed).toEqual(true)
  });

  it('should render download controls', () => {
    let store = new FileTransferStore()

    store = store.merge({
      isOpen: true,
      isUpload: false
    });

    const onClose = () => { }
    const onTransfer = () => { }

    render({
      store,
      onClose,
      onTransfer
    });

    expect($node.find('.grv-file-transfer-download').length).toEqual(1)
    expect($node.find('.grv-file-transfer-btn:disabled').length).toEqual(1)
  });

  const render = props => {
    return ReactDOM.render((
      <FileTransferDialog {...props}
        />
    ), $node[0]);
  }

});