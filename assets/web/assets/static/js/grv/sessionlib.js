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
function sortByActivity(x, y) {
    if(!x.was_active && !y.was_active) {
        return 0;
    }
    if(!y.was_active) {
        return -1;
    }                
    if(!x.was_active) {
        return 1;
    }
    if (x.last_active < y.last_active) {
        return 1;
    }
    if (x.last_active > y.last_active) {
        return -1;
    }
    return 0;
}

function parseSessionWithServers(se, servers) {
    var session = {
        id: se.id,
        servers: [],
        parties: [],
        first_server: se.first_server || "",
    };

    var smap = {};
    for (var i = 0; i < servers.length; i ++ ) {
        smap[servers[i].addr] = servers[i];
    }

    var sa = {};
    
    for(var i = 0; i < se.parties.length; ++i) {
        var p = se.parties[i];
        p.last_active = new Date(p.last_active);
        p.was_active = true;
        p.server = smap[p.server_addr];
        session.parties.push(p);

        if(!sa.hasOwnProperty(p.server_addr)) {
            sa[p.server_addr] = p.last_active;
        }
        if(sa[p.server_addr] < p.last_active) {
            sa[p.server_addr] = p.last_active;
        }
    }

    for (var i = 0; i < servers.length; i ++ ) {
        var srv = servers[i];
        srv.was_active = sa.hasOwnProperty(srv.addr);
        srv.last_active = sa[srv.addr];
        session.servers.push(srv);
    }

    session.servers.sort(sortByActivity);
    session.parties.sort(sortByActivity);

    return {session: session};
}

function parseSession(se) {
    var d = new Date();
    d.setHours(d.getHours() - 1);
    var session = {
        id: se.id,
        parties: [],
        users: [],
        last_active: d
    };

    var users = {};

    for(var i = 0; i < se.parties.length; ++i) {
        var p = se.parties[i];
        var last_active = new Date(p.last_active);
        if(session.last_active < last_active) {
            session.last_active = last_active;
        }
        users[p.user] = true;
    }

    for(var user in users) {
        session.users.push(user);
    }

    return session;
}
