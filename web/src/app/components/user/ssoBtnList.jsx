import React from 'react';
import classnames from 'classnames';
import { AuthProviderEnum, AuthProviderTypeEnum } from 'app/services/enums';

const ProviderIcon = ({ provider }) => {
  let { name, type } = provider;

  let iconClass = classnames('fa', {
    'fa-google': name === AuthProviderEnum.GOOGLE,
    'fa-windows': name === AuthProviderEnum.MS,
    'fa-github': name === AuthProviderEnum.GITHUB || AuthProviderTypeEnum.GITHUB,
    'fa-bitbucket': name === AuthProviderEnum.BITBUCKET
  });

  // do not render any icon for unknown SAML providers
  if (iconClass === 'fa' && type === AuthProviderTypeEnum.SAML) {
    return null;
  }
  
  // use default oidc icon for unknown oidc providers
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
    let { name, displayName } = item;    
    displayName = displayName || name;
    let title = `${prefixText} ${displayName}`
    let providerBtnClass = getProviderBtnClass(name);
    let btnClass = `btn grv-user-btn-sso full-width ${providerBtnClass}`;
    return (
      <button key={index}
        disabled={isDisabled}
        className={btnClass}
        onClick={e => { e.preventDefault(); onClick(item) }}>      
        <ProviderIcon provider={item}/>
        <span>{title}</span>      
      </button>              
    )
  })
  
  if ($btns.length === 0) {
    return (
      <h4> You have no SSO providers configured </h4>
    )
  }

  return (
    <div> {$btns} </div>
  )
}

export { SsoBtnList }
