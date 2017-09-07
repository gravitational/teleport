## web client

### To build and regenerate /dist/* files

With docker

```
make
```

Locally 

```
install nodejs >= 8.0.0
```
```
make local
```


To run a dev server

```
npm run start
```
```
open https://localhost:8081/web
```

### To generate a list of NPM modules with their license 

```
npm install -g nlf
```

```
nlf -d --summary detail
```