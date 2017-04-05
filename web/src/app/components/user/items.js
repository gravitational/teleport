import React from 'react';
const U2F_ERROR_CODES_URL = 'https://developers.yubico.com/U2F/Libraries/Client_error_codes.html';

export const ErrorMessage = ({ message }) => {
  message = message || '';
  if(message.indexOf('U2F') !== -1 ) {
    return (
      <label className="grv-invite-login-error">
        {message}
        <br />
        <small className="grv-invite-login-error-u2f-codes">
          <span>click <a target="_blank" href={U2F_ERROR_CODES_URL}>here</a> to learn more about U2F error codes
            </span>
        </small>
      </label>
    )
  }
    
  return (
    <label className="error">{message} </label>
  )
}