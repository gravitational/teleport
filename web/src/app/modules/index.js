var reactor = require('app/reactor');
reactor.registerStores({
  'tlpt': require('./app/appStore'),
  'tlpt_dialogs': require('./dialogs/dialogStore'),
  'tlpt_current_session': require('./activeTerminal/activeTermStore'),
  'tlpt_user': require('./user/userStore'),
  'tlpt_nodes': require('./nodes/nodeStore'),
  'tlpt_invite': require('./invite/inviteStore'),
  'tlpt_rest_api': require('./restApi/restApiStore'),
  'tlpt_sessions': require('./sessions/sessionStore'),
  'tlpt_stored_sessions_filter': require('./storedSessionsFilter/storedSessionFilterStore'),
  'tlpt_notifications': require('./notifications/notificationStore')
});
