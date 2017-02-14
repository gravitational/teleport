import {
  ReactTestUtils,
  React,
  ReactDOM,  
  $,
  expect,  
  reactor
} from 'app/__tests__/';

import enums from 'app/services/enums';

import { InviteInputForm } from 'app/components/user/invite';

let $node = $('<div>').appendTo("body");

describe('components/user/invite', () => {

  afterEach(function () {
    ReactDOM.unmountComponentAtNode($node[0]);
    expect.restoreSpies();
    reactor.reset();
  })
  
  it('should handle AuthTypeEnum.LOCAL', () => {        
    let props = getProps({
      authType: enums.AuthTypeEnum.LOCAL
    });
        
    render(props);        
    let expected = ['psw1234', ''];
        
    setValues(...expected);    
    clickLogin();    
    expectNInputs(4);
    expect(props.onSignup).toHaveBeenCalledWith(props.invite.user, ...expected);    
  });    

  it('should handle AuthTypeEnum.LOCAL with Auth2faTypeEnum.UTF', () => {                            
    let props = getProps({
      authType: enums.AuthTypeEnum.LOCAL,      
      auth2faType: enums.Auth2faTypeEnum.UTF      
    });
    
    render(props);    

    let expected = ['psw1234', ''];

    setValues(...expected);        
    clickLogin();
    expectNInputs(4);            
    expect(props.onSignupWithU2f).toHaveBeenCalledWith(props.invite.user, expected[0]);    
  });    

  it('should handle AuthTypeEnum.LOCAL with Auth2faTypeEnum.OTP', () => {                    
    let props = getProps({
      authType: enums.AuthTypeEnum.LOCAL,      
      auth2faType: enums.Auth2faTypeEnum.OTP      
    });
    
    render(props);    
    
    let expected = ['psw1234', 'token'];        

    setValues(...expected);        
    clickLogin();
    expectNInputs(5);            
    expect(props.onSignup).toHaveBeenCalledWith(props.invite.user, ...expected);    
  });    

  it('should handle AuthTypeEnum.OIDC', () => {        
    let props = getProps({
      authType: enums.AuthTypeEnum.OIDC,
      authProvider: enums.AuthProviderEnum.MS
    });

    render(props);    
                
    $node.find(".btn-microsoft").click();    
    expectNInputs(1);
    expect(props.onSignupWithOidc).toHaveBeenCalledWith(enums.AuthProviderEnum.MS);    
  });    
  
  it('should handle AuthTypeEnum.OIDC with Auth2faTypeEnum.OTP', () => {                
    let props = getProps({
      authType: enums.AuthTypeEnum.OIDC,
      authProvider: enums.AuthProviderEnum.MS,
      auth2faType: enums.Auth2faTypeEnum.OTP      
    });
          
    render(props);    
    expectNInputs(6);        

    $node.find(".btn-microsoft").click();    
    expect(props.onSignupWithOidc).toHaveBeenCalledWith(enums.AuthProviderEnum.MS);    

    let expected = ['psw123', 'token'];        
    setValues(...expected);        
    clickLogin();
    expect(props.onSignup).toHaveBeenCalledWith(props.invite.user, ...expected);    
  });    

  it('should handle AuthTypeEnum.OIDC with Auth2faTypeEnum.UTF', () => {                
    let props = getProps({
      authType: enums.AuthTypeEnum.OIDC,
      authProvider: enums.AuthProviderEnum.MS,
      auth2faType: enums.Auth2faTypeEnum.UTF      
    });

    render(props);    
    expectNInputs(5);        
  });    

});

const setValues = (password, token) => {  
  if (password) {
    setText($node.find('input[name="password"]')[0], password);
    setText($node.find('input[name="passwordConfirmed"]')[0], password);
  }

  if (token) {
    setText($node.find('input[name="token"]')[0], token);
  }
}
  
const clickLogin = () => {
  $node.find(".btn-primary").click();
}

const expectNInputs = n => {
  expect($node.find('input, button').length).toBe(n);
}

const getProps = customProps => {
  let props = {
    authProvider: '',    
    auth2faType: '',    
    authType: '',    
    onSignupWithOidc(/*providerName*/) { },
    onSignupWithU2f(/*username, password*/) { },
    onSignup(/*username, password, token*/) { },    
    invite: {
      user: 'test_user'
    },
    attemp: { },
    ...customProps
  };

  expect.spyOn(props, 'onSignupWithOidc');
  expect.spyOn(props, 'onSignupWithU2f');
  expect.spyOn(props, 'onSignup');

  return props;
}

 
function render(props){
  return ReactDOM.render(<InviteInputForm {...props }/>, $node[0]);
}

function setText(el, val){
  ReactTestUtils.Simulate.change(el, { target: { value: val } });
}
