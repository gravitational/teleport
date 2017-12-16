import React from 'react';

import { AuthProviderTypeEnum } from 'app/services/enums';

const guessProviderBtnClass = (name, type) => {  
  name = name.toLowerCase();

  if (name.indexOf('microsoft') !== -1) {
    return 'btn-microsoft';    
  }

  if (name.indexOf('bitbucket') !== -1) {
    return 'btn-bitbucket';  
  }
  
  if (name.indexOf('google') !== -1) {
    return 'btn-google';
  }

  if (name.indexOf('github') !== -1 || type === AuthProviderTypeEnum.GITHUB ) {
    return 'btn-github';
  }

  if (type === AuthProviderTypeEnum.OIDC) {
    return 'btn-openid';
  }

  return '--unknown';   
}

const SsoBtnList = ({providers, prefixText, isDisabled, onClick}) => {      
  const $btns = providers.map((item, index) => {
    let { name, type, displayName } = item;    
    displayName = displayName || name;
    const title = `${prefixText} ${displayName}`
    const providerBtnClass = guessProviderBtnClass(displayName, type);
    const btnClass = `btn grv-user-btn-sso full-width ${providerBtnClass}`;
    return (
      <button key={index}
        disabled={isDisabled}
        className={btnClass}
        onClick={e => { e.preventDefault(); onClick(item) }}>              
        <div className="--sso-icon">
          <span className="fa"/>
        </div>
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
