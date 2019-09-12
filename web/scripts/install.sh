#!/bin/bash
package_json_update_time=`(date -r package.json "+%s")`
node_modules_update_time=`(date -r node_modules "+%s")`
if ! [[ $node_modules_update_time =~ ^[0-9]+$ ]] || [ $package_json_update_time -gt $node_modules_update_time ];
 then
   yarn install
 else
   echo "package.json and node_modules are in sync"
fi