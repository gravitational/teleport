module.exports.isMatch = function(obj, searchValue, {searchableProps, cb}) {
  searchValue = searchValue.toLocaleUpperCase();
  let propNames = searchableProps || Object.getOwnPropertyNames(obj);
  for (let i = 0; i < propNames.length; i++) {
    let targetValue = obj[propNames[i]];
    if (targetValue) {
      if(typeof cb === 'function'){
        let result = cb(targetValue, propNames[i]);
        if(result !== undefined){
          return result;
        }
      }

      if (targetValue.toString().toLocaleUpperCase().indexOf(searchValue) !== -1) {
        return true;
      }
    }
  }

  return false;
}
