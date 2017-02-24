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
import reactor from 'app/reactor';
import { lastMessage } from 'app/modules/notifications/getters';
import { ToastContainer, ToastMessage } from "react-toastr";

const ToastMessageFactory = React.createFactory(ToastMessage.animation);

const animationOptions = {
  showAnimation: 'animated fadeIn',
  hideAnimation: 'animated fadeOut'
}

const NotificationHost = React.createClass({

  mixins: [ reactor.ReactMixin ],

  getDataBindings() {
    return {msg: lastMessage}
  },

  update(msg) {
    if (msg) {
      if (msg.isError) {
        this.refs.container.error(msg.text, msg.title, animationOptions);
      } else if (msg.isWarning) {
        this.refs.container.warning(msg.text, msg.title, animationOptions);
      } else if (msg.isSuccess) {
        this.refs.container.success(msg.text, msg.title, animationOptions);
      } else {
        this.refs.container.info(msg.text, msg.title, animationOptions);
      }
    }
  },

  componentDidMount() {
    reactor.observe(lastMessage, this.update)
  },

  componentWillUnmount() {
    reactor.unobserve(lastMessage, this.update);
  },

  render() {
    return (
      <ToastContainer
        className="toast-top-right"  
        ref="container"
        toastMessageFactory={ToastMessageFactory} />
    );
  }

});

export default NotificationHost;