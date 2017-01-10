## Web UI

Teleport UI is written in Javascript using (React)[https://facebook.github.io/react/]  
Building the UI:

```bash
$ cd <teleport repo>/web
$ install nodejs >= 5.0.0
$ npm install
$ npm run build
```
This will create `dist` directory with `index.html` + app assets.

To run a dev server:

1. `npm run start`
2. `open https://localhost:8081/web`

To run using Teleport binary (it will load web assets from `web/app/dist`):

```
$ cd <teleport repo>
$ DEBUG=1 build/teleport start -d
```
