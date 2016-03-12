var moment = require('moment');

module.exports.monthRange = function(value = new Date()){
  let startDate = moment(value).startOf('month').toDate();
  let endDate = moment(value).endOf('month').toDate();
  return [startDate, endDate];
}
