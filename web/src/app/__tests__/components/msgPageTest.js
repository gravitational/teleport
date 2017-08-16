import React from 'react';
import { Router, createMemoryHistory } from 'react-router';
import expect from 'expect';
import DocumentTitle from 'app/components/documentTitle';
import cfg from 'app/config';
import * as Messages from 'app/components/msgPage'
import { $ } from 'app/__tests__/';
import { makeHelper, ReactDOM } from 'app/__tests__/domUtils';

const $node = $('<div>');
const helper = makeHelper($node);

let rootRoutes = [  
  {
    component: DocumentTitle,
    childRoutes: [  
      { path: cfg.routes.error, component: Messages.ErrorPage },
      { path: cfg.routes.info, component: Messages.InfoPage }    
    ]
  }
]

describe('components/msgPage', function () {
    
  const history = new createMemoryHistory();   

  beforeEach(()=>{
    helper.setup()
  });

  afterEach(() => {    
    helper.clean();    
  })
  
  it('should render default error', () => {                                         
    history.push('/web/msg/error')
    render();    
    expectHeaderText(Messages.MSG_ERROR_DEFAULT)
    expectDetailsText('')
  });
        
  it('should render login failed', () => {                                         
    history.push('/web/msg/error/login_failed?details=test')
    render();
    expectHeaderText(Messages.MSG_ERROR_LOGIN_FAILED)
    expectDetailsText('test')
  });

  it('should render expired invite', () => {                                         
    history.push('/web/msg/error/expired_invite')
    render();
    expectHeaderText(Messages.MSG_ERROR_EXPIRED_INVITE)    
  });
  
  it('should render not found', () => {                                         
    history.push('/web/msg/error/not_found')
    render();
    expectHeaderText(Messages.MSG_ERROR_NOT_FOUND)        
  });

  it('should render access denied', () => {                                         
    history.push('/web/msg/error/access_denied')
    render();
    expectHeaderText(Messages.MSG_ERROR_ACCESS_DENIED)        
  });

  it('should render login succesfull', () => {                                         
    history.push('/web/msg/info/login_success')
    render();
    expectHeaderText(Messages.MSG_INFO_LOGIN_SUCCESS)
  });
    
  const expectDetailsText = text => {
    expect($node.find('.grv-msg-page-details-text').text()).toEqual(text)
  }

  const expectHeaderText = text => {
    expect($node.find('.grv-msg-page h1:first').text()).toEqual(text)
  }

  const render = () => {    
    ReactDOM.render(( <Router history={history} routes={rootRoutes} /> ) , $node[0]);  
  }

});

