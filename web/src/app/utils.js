var utils = {

  uuid(){
    // never use it in production
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
      var r = Math.random()*16|0, v = c == 'x' ? r : (r&0x3|0x8);
      return v.toString(16);
    });
  },

  displayDate(date){
    try{
      return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
    }catch(err){
      console.error(err);
      return 'undefined';
    }
  },

  formatString(format) {
    var args = Array.prototype.slice.call(arguments, 1);
    return format.replace(new RegExp('\\{(\\d+)\\}', 'g'),
      (match, number) => {
        return !(args[number] === null || args[number] === undefined) ? args[number] : '';
    });
  }
            
}

module.exports = utils;
