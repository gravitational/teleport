var reactor = require('app/reactor');
reactor.registerStores({
  'tlpt_user': require('./user/userStore'),
  'tlpt_nodes': require('./nodes/nodeStore'),
  'tlpt_invite': require('./invite/inviteStore'),
  'tlpt_rest_api': require('./restApi/restApiStore')
});
