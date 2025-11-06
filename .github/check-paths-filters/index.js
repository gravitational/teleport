const fs = require("node:fs");
const path = require("node:path");
const yaml = require("yaml");

const workflows = fs.readdirSync(path.join("..", "workflows");
workflows.forEach(wf=>{
    if(path.extname(wf) !== ".yaml"){
    	return
    }

    const file = fs.readFileSync(wf, {
	encoding: "utf8"
    });
    
    const config = yaml.parse(file);
    if(!config.hasOwnProperty("jobs")){
    	return
    }

    // TODO: check each job for if it has a step with paths-filter
});

