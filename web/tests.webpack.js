var context = require.context('./src/app/__tests__/', true, /\Test.js$/);
context.keys().forEach(context);
