import expect from 'expect';
import $ from 'jQuery';
import ReactTestUtils from 'react-addons-test-utils';
import { React, makeHelper, ReactDOM } from 'app/__tests__/domUtils';
import InputSshServer from 'app/components/inputSshServer';
import { SshHistoryRec } from 'app/flux/sshHistory/store';

const $node = $('<div>');
const helper = makeHelper($node);

describe('components/inputSshLogin', function () {
      
  beforeEach(()=>{
    helper.setup()
  });

  afterEach(() => {        
    helper.clean();    
  })
  
  it('should render', () => {                                         
    let store = new SshHistoryRec()
    store = store.addLoginString({ siteId: 'cluster1', serverId: 'one', login: 'root' })    
    render(store, 'cluster1', () => { });              
    expectRenderedCmpt();
    expectNoValidationErrors();    
  });
  
  it('should handle up and down keys', () => {                                         
    let store = new SshHistoryRec()    
    store = store.addLoginString({ siteId: 'cluster1', serverId: 'one', login: 'root' })    
    store = store.addLoginString({ siteId: 'cluster1', serverId: 'two', login: 'root' })    
    let cmpt = render(store, 'cluster1', () => { });              
                  
    keyUp(cmpt.inputRef);
    expect(cmpt.inputRef.value).toBe('root@two');

    keyUp(cmpt.inputRef);
    expect(cmpt.inputRef.value).toBe('root@one');    
    
    keyUp(cmpt.inputRef);    
    expect(cmpt.inputRef.value).toBe('root@one', "should not change");    
    
    keyDown(cmpt.inputRef);    
    expect(cmpt.inputRef.value).toBe('root@two');    

    keyDown(cmpt.inputRef);    
    expect(cmpt.inputRef.value).toBe('');    
  });
    
  it('should handle cluster change', () => {    
    let store = new SshHistoryRec();    
    store = store.addLoginString({ siteId: 'cluster1', serverId: 'one', login: 'root' })            
    let cmpt = render(store, 'cluster1', () => { });              
                     
    keyUp(cmpt.inputRef);    
    expect(cmpt.inputRef.value).toBe('root@one');
    
    cmpt = render(store, 'cluster2', () => { });              
    expect(cmpt.inputRef.value).toBe('', 'should reset its value to empty');
  });

  it('should handle onEnter', () => {
    let store = new SshHistoryRec()    
    let expectedLogin = 'login';
    let expectedServer = 'one';
    let actualLogin = '';
    let actualServer = '';
    
    let cmpt = render(store, 'cluster1', (login, server) => { 
      actualLogin = login;
      actualServer = server;
    });

    setValue(cmpt.inputRef, `${expectedLogin}@${expectedServer}`);
    keyPress(cmpt.inputRef);
    
    expect(actualLogin).toBe(expectedLogin);
    expect(actualServer).toBe(expectedServer);
  });
            
  const expectRenderedCmpt = () => {
    expect($node.find('.grv-sshserver-input').length).toEqual(1)
  }

  const expectNoValidationErrors = () => {
    expect($node.find('.--error').length).toEqual(0)
  }

  const keyUp = cmpt => {
    ReactTestUtils.Simulate.keyUp(cmpt, { which: 38 });        
  }

  const keyDown = cmpt => {
    ReactTestUtils.Simulate.keyUp(cmpt, { which: 40 });        
  }

  const setValue = (cmpt, val) => {
    ReactTestUtils.Simulate.change(cmpt, { target: { value: val } });
  }

  const keyPress = cmpt => {
    ReactTestUtils.Simulate.keyPress(cmpt, { key: 'Enter' });
  }

  const render = (store, siteId, onEnter) => {    
    return ReactDOM.render((
      <InputSshServer
        autoFocus={true}
        clusterId={siteId}
        sshHistory={store}
        onEnter={onEnter} />
    ), $node[0]);     
  }
});

