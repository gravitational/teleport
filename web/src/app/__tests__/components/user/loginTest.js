import {
  ReactTestUtils,
  React,
  ReactDOM,  
  $,
  expect,  
  reactor
} from 'app/__tests__/';

import enums from 'app/services/enums';

import { LoginInputForm } from 'app/components/user/login';

let $node = $('<div>').appendTo("body");

describe('components/user/login', () => {

  afterEach(function () {
    ReactDOM.unmountComponentAtNode($node[0]);
    expect.restoreSpies();
    reactor.reset();
  })
  
  it('should handle AuthTypeEnum.LOCAL', () => {        
    let props = getProps({
      authType: enums.AuthTypeEnum.LOCAL
    });

    render(<LoginInputForm { ...props } />);    
    
    let expected = ['email@email', '123456', ''];
        
    setValues(...expected);    
    clickLogin();    
    expectNInputs(3);
    expect(props.onLogin).toHaveBeenCalledWith(...expected);    
  });    

  it('should handle AuthTypeEnum.LOCAL with Auth2faTypeEnum.UTF', () => {                    
    let props = getProps({
      authType: enums.AuthTypeEnum.OIDC,
      authProviders: [{ name: enums.AuthProviderEnum.MS }],
      auth2faType: enums.Auth2faTypeEnum.UTF      
    });
    
    render(<LoginInputForm { ...props } />);                            

    let expected = ['email@email', '123456'];        

    setValues(...expected);        
    clickLogin();
    expectNInputs(4);            
    expect(props.onLoginWithU2f).toHaveBeenCalledWith(...expected);    
  });    

  it('should handle AuthTypeEnum.LOCAL with Auth2faTypeEnum.OTP', () => {                    
    let props = getProps({
      authType: enums.AuthTypeEnum.LOCAL,      
      auth2faType: enums.Auth2faTypeEnum.OTP      
    });
    
    render(<LoginInputForm { ...props } />);                            

    let expected = ['email@email', '123456', 'token'];        

    setValues(...expected);        
    clickLogin();
    expectNInputs(4);            
    expect(props.onLogin).toHaveBeenCalledWith(...expected);    
  });    

  it('should handle AuthTypeEnum.OIDC', () => {        
    let props = getProps({
      authType: enums.AuthTypeEnum.OIDC,
      authProviders: [{ name: enums.AuthProviderEnum.MS }]
    });

    render(<LoginInputForm { ...props } />);    
                
    $node.find(".btn-microsoft").click();    
    expectNInputs(1);
    expect(props.onLoginWithOidc).toHaveBeenCalledWith(enums.AuthProviderEnum.MS);    
  });    
  
  it('should handle AuthTypeEnum.OIDC with Auth2faTypeEnum.OTP', () => {                
    let props = getProps({
      authType: enums.AuthTypeEnum.OIDC,
      authProviders: [{ name: enums.AuthProviderEnum.MS }],
      auth2faType: enums.Auth2faTypeEnum.OTP      
    });
      
    render(<LoginInputForm { ...props } />);                            
    expectNInputs(5);        

    $node.find(".btn-microsoft").click();    
    expect(props.onLoginWithOidc).toHaveBeenCalledWith(enums.AuthProviderEnum.MS);    

    let expected = ['email@email', '123456', 'token'];        
    setValues(...expected);        
    clickLogin();
    expect(props.onLogin).toHaveBeenCalledWith(...expected);    
  });    

  it('should handle AuthTypeEnum.OIDC with Auth2faTypeEnum.UTF', () => {                
    let props = getProps({
      authType: enums.AuthTypeEnum.OIDC,
      authProviders: [{ name: enums.AuthProviderEnum.MS }],
      auth2faType: enums.Auth2faTypeEnum.UTF      
    });

    render(<LoginInputForm { ...props } />);                            
    expectNInputs(4);        
  });    

});

const setValues = (user, password, token) => {
  if (user) {
    setText($node.find('input[name="userName"]')[0], user);
  }

  if (password) {
    setText($node.find('input[name="password"]')[0], password);
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
    authProviders: [],    
    auth2faType: '',    
    authType: '',    
    onLoginWithOidc(/*providerName*/) { },
    onLoginWithU2f(/*username, password*/) { },
    onLogin(/*username, password, token*/) { },    
    attemp: { },
    ...customProps
  };

  expect.spyOn(props, 'onLoginWithOidc');
  expect.spyOn(props, 'onLoginWithU2f');
  expect.spyOn(props, 'onLogin');

  return props;
}

 
function render(component){
  return ReactDOM.render(component, $node[0]);
}

function setText(el, val){
  ReactTestUtils.Simulate.change(el, { target: { value: val } });
}
