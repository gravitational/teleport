import * as enums from 'app/services/enums';
import { InviteInputForm } from 'app/components/user/invite';
import {
  ReactTestUtils,
  React,
  ReactDOM,  
  $,
  expect,  
  reactor
} from 'app/__tests__/';

let $node = $('<div>').appendTo("body");

describe('components/user/invite', () => {

  afterEach(function () {
    ReactDOM.unmountComponentAtNode($node[0]);
    expect.restoreSpies();
    reactor.reset();
  })
  
  it('should sign-up with username and password', () => {        
    let props = getProps({      
      auth2faType: enums.Auth2faTypeEnum.DISABLED
    });
        
    render(props);        
    let expected = ['psw1234', ''];
        
    setValues(...expected);    
    clickLogin();    
    expectNInputs(4);
    expect(props.onSubmit).toHaveBeenCalledWith(props.invite.user, ...expected);    
  });    

  it('should sign-up with Auth2faTypeEnum.UTF', () => {                            
    let props = getProps({      
      auth2faType: enums.Auth2faTypeEnum.UTF      
    });
    
    render(props);    

    let expected = ['psw1234', ''];

    setValues(...expected);        
    clickLogin();
    expectNInputs(4);            
    expect(props.onSubmitWithU2f).toHaveBeenCalledWith(props.invite.user, expected[0]);    
  });    

  it('should sign-up with Auth2faTypeEnum.OTP', () => {                    
    let props = getProps({      
      auth2faType: enums.Auth2faTypeEnum.OTP      
    });
    
    render(props);    
    
    let expected = ['psw1234', 'token'];        

    setValues(...expected);        
    clickLogin();
    expectNInputs(5);            
    expect(props.onSubmit).toHaveBeenCalledWith(props.invite.user, ...expected);    
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
    auth2faType: '',    
    authType: '',        
    onSubmitWithU2f(/*username, password*/) { },
    onSubmit(/*username, password, token*/) { },    
    invite: {
      user: 'test_user'
    },
    attemp: { },
    ...customProps
  };
  
  expect.spyOn(props, 'onSubmitWithU2f');
  expect.spyOn(props, 'onSubmit');

  return props;
}

 
function render(props){
  return ReactDOM.render(<InviteInputForm {...props }/>, $node[0]);
}

function setText(el, val){
  ReactTestUtils.Simulate.change(el, { target: { value: val } });
}
