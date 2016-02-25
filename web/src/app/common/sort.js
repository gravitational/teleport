module.exports = function sort(data, params){
  return data.sort((a, b)=>{
    for (var i = 0; i < params.length; i++) {
        if (a[params[i].name] === b[params[i].name]) {
            continue;
        }

        let valueA = a[params[i].name];
        let valueB = b[params[i].name];
        let dir = params[i].desc === true ? -1 : 1;
        let isString = false;

        if (typeof valueA === 'string' && typeof valueB === 'string') {
            valueA = valueA.toLocaleUpperCase();
            valueB = valueB.toLocaleUpperCase();
            isString = true;
        }

        if (isNullOrUndefined(valueA)) {
            return -1 * dir;
        }

        if (isNullOrUndefined(valueB)) {
            return dir;
        }

        if (isString) {
            var priority = valueA.localeCompare(valueB);
            return priority === 0 ? priority : dir * priority;
        }

        return valueA > valueB ? dir : -1 * dir;
    }

    return 0;
  });
};

function isNullOrUndefined(obj) {
  return obj === null || obj === undefined;
}
