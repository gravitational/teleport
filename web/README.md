## web client

Build (to create new /dist files)

1. `install nodejs >= 5.0.0`
2. `npm install`
3. `npm run build`

To run a dev server (development)

1. `npm run start`
2. `open https://localhost:8081/web`

NOTE: use `CGO_ENABLED=true make release` if you see the following when running teleport:
```
FATA[0000] zip: not a valid zip file                     file=web/static.go:107
```
