## web client

### To build and regenerate /dist/* files

With docker

```
$ make
```

Locally 

```
install nodejs >= 8.0.0
```
```
$ make local
```


To run a dev server

```
$ npm run start
```
```
open https://localhost:8081/web
```

### To generate a list of dependencies with their licenses 

```
$ npm install -g nlf
$ nlf -d --summary detail > licenses.txt
```

Then modify the file by excluding dual dependencies and adding manual dependecies from /assets/vendor folder

If you want to generate JSON file to build custom reports.
https://github.com/davglass/license-checker

```
$ npm install -g license-checker
$ license-checker --production --json --summary > licenses.json
```
