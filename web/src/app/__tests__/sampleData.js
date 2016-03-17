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

var ids = {
  serverIds: ['ad2109a6-42ac-44e4-a570-5ce1b470f9b6'],
  sids: ['f60c4f1e-aedd-4fa6-8fe5-8068b49b17b4', '11d76502-0ed7-470c-9ae2-472f3873fa6e']
}

module.exports.ids = ids;

module.exports.user = {
  "name":"alex",
  "allowed_logins":["admin","bob"]
}

module.exports.sessions = {
  sessions: [{
    "id": ids.sids[0],
    "parties": null,
    "terminal_params": {
      "w": 115,
      "h": 34
    },
    "login": "akontsevoy",
    "active": false,
    "created": "2016-03-12T20:25:02.748578423Z",
    "last_active": "2016-03-12T20:25:02.748578518Z"
  },
  {
    "id": ids.sids[1],
    "parties": [{
      "id": "66dfccf2-867f-4835-a337-8d5a241365ed",
      "remote_addr": "127.0.0.1:60973",
      "user": "user1",
      "server_id": "ad2109a6-42ac-44e4-a570-5ce1b470f9b6",
      "last_active": "2016-03-15T15:55:49.306916333-04:00"
    },
    {
      "id": "66dfccf2-867f-4835-a337-8d5a241365e2",
      "remote_addr": "127.0.0.1:60973",
      "user": "user2",
      "server_id": "ad2109a6-42ac-44e4-a570-5ce1b470f9b6",
      "last_active": "2016-03-15T15:56:49.306916333-04:00"
    }],
    "terminal_params": {
      "w": 114,
      "h": 36
    },
    "login": "akontsevoy",
    "active": true,
    "created": "2016-03-15T19:55:49.251601013Z",
    "last_active": "2016-03-15T19:55:49.251601164Z"
  }]
},

module.exports.nodes = {
  "nodes": [{
    "node": {
      "id": ids.serverIds[0],
      "addr": "0.0.0.0:3022",
      "hostname": "x220",
      "labels": {
        "role": "mysql"
      },
      "cmd_labels": {
        "db_status": {
          "command": "mysql -c status",
          "result": "master",
          "period": 1000000000
        }
      },
    }
  }]
};
