import React from 'react';
import classnames from 'classnames';
import { AuthProviderEnum } from 'app/services/enums';

const ProviderIcon = ({ type }) => {

  let iconClass = classnames('fa', {
    'fa-google': type === AuthProviderEnum.GOOGLE,
    'fa-windows': type === AuthProviderEnum.MS,
    'fa-github': type === AuthProviderEnum.GITHUB,
    'fa-bitbucket': type === AuthProviderEnum.BITBUCKET
  });

  if (iconClass === 'fa') {
    iconClass = `${iconClass} fa-openid`;
  }

  return (
    <div className="--sso-icon">
      <span className={iconClass}></span>
    </div>
  )
}

const getProviderBtnClass = type => {  
  switch (type) {
    case AuthProviderEnum.BITBUCKET:
      return 'btn-bitbucket';  
    case AuthProviderEnum.GITHUB:
      return 'btn-github';  
    case AuthProviderEnum.MS:
      return 'btn-microsoft';    
    case AuthProviderEnum.GOOGLE:
      return 'btn-google';
    default:
      return 'btn-openid'; 
  }    
}

const SsoBtnList = ({providers, prefixText, isDisabled, onClick}) => {      
  let $btns = providers.map((item, index) => {
    let { name, display } = item;    
    display = display || name;
    let title = `${prefixText} ${display}`
    let providerBtnClass = getProviderBtnClass(name);
    let btnClass = `btn grv-user-btn-sso full-width ${providerBtnClass}`;
    return (
      <button key={index}
        disabled={isDisabled}
        className={btnClass}
        onClick={e => { e.preventDefault(); onClick(name) }}>      
        <ProviderIcon type={name}/>
        <span>{title}</span>      
      </button>              
    )
  })
  
  if ($btns.length === 0) {
    return (
      <h4> You have no configured OIDC providers </h4>
    )
  }

  return (
    <div> {$btns} </div>
  )
}

export { SsoBtnList }
