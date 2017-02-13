/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

const GoogleAuthInfo = () => {
  return (
    <div className="grv-google-auth text-left">
      <div className="grv-icon-google-auth"/>
      <strong>Google Authenticator</strong>
      <div>Download 
        <a href="https://support.google.com/accounts/answer/1066447?hl=en">
          <span> Google Authenticator </span>
        </a>
        on your phone to access your two factor token</div>
    </div>
  );
}

export default GoogleAuthInfo;