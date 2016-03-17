var moment = require('moment');

module.exports.weekRange = function(value = new Date()){
  let startDate = moment(value).startOf('week').toDate();
  let endDate = moment(value).endOf('week').toDate();
  return [startDate, endDate];
}
