module.exports.nodesAndSessions = {
  "nodes": [{
    "node": {
      "id": "0.0.0.0:3022",
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
    },
    "sessions": [{
      "id": "7d8dc291-6144-485b-aa53-6b1ef34daa93",
      "parties": [{
        "id": "6c65953f-eadd-44e8-bf32-6a28f14804cb",
        "site": "127.0.0.1:37876",
        "user": "user1",
        "server_addr": "0.0.0.0:3022",
        "last_active": "2016-02-27T17:52:09.035902892-05:00"
      }, {
        "id": "72a71c49-8f24-4fa8-b47d-3e6ce3104f9c",
        "site": "127.0.0.1:37887",
        "user": "user2",
        "server_addr": "0.0.0.0:3022",
        "last_active": "2016-02-27T18:13:17.008705077-05:00"
      }]
    }]
  }]
};
