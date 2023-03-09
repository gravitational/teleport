---
authors: Sai Sandeep Rangisetti (sandeep.sai@flipkart.com)
state: draft
---

# RFD 112 - Use a credential provider for DBs

## What
As of now all the self-hosted databases require enabling certificate authentication in the backend. This RFD proposes
a way to avoid that requirement. Based on the labels we can decide to fetch credentials from a web service instead of
using certificates for authentication. When this label is applied we will consider username as a token and exchange it
for credentials with an external credential provider

## Why
Enabling certificate authentication requires config update and restart of the mysql server. This will not be scalable in
large organizations which will have large number of database clusters. Instead, orgs can have a credential provider
service like Vault which can give the credentials on need basis based on username and database. 

## Detail
### Overview 
```
+--------------+               +--------------------+               +------------------+
|              | pass token as |                    | pass exhanged |                  |
|     user     +-------------->|      teleport      +-------------->|     database     |
|              |   username    |                    |  credentials  |                  |
+--------------+               +--------+-----------+               +------------------+
                                        |  ^
                                        |  |
                                        |  | Exchange token with credentials
                                        V  |
                               +-----------+--------+
                               |                    |
                               |     credential     |
                               |      provider      |
                               |                    |
                               +--------------------+

```

### Get the details of the credential provider
To exchange the username/token with actual credentials we need to read the required details like url, authentication
mechanism and other details. We can read the details from the config file like below 
```yaml
db_service:
  token:
    url_tmpl: http://localhost:9000/database/{{.DBName}}/user/{{.Username}}/token/{{.Token}}
    connection_timeout: 1s
    read_timeout: 1s
    tls:
      insecure: false
      # ca path is required if server is cert is self signed
      ca_path: /tmp/ca.pem
      # cert and key path required if authentication is based on mutual tls
      cert_path: /tmp/cert.pem
      key_path: /tmp/key.pem
    authentication:
      # scheme could be NONE or HEADER or IDTOKEN if it is header we will send fixed set of headers
      scheme: HEADER
      headers:
        X-API-Key: knkncsdjknkn
```
### Exchange the credentials for the token
As of now teleport has conditions for cloud provided DBs to fetch credentials in various db engines. In similar fashion
we can have one more condition for token enabled DBs to exchange credentials.
#### Sample request:
```bash
curl --location http://localhost:9000/database/test-db/user/ssrangisetti/token/test-token \
--header 'X-API-Key: knkncsdjknkn' `# X-API-KEY header added only if authentication scheme is HEADER` \
--header 'Authorization: Bearer <bearer token>' # Authorization header is added only if authentication scheme is IDTOKEN
```
#### Sample response:
```json5
{
  "data": {
    // username and password if the response is successful
    "username": "ajlndsc",
    "password": "kmndckl"
  },
  // error code and message if the response is not successful
  "errors": ["some error"]
}
```

## UX
* Users while onboarding a new database don't have to do anything
* Users need to use token provided by credential provider like below
  * `tsh db connect --db-user <token> <db name>`
  * `tsh proxy db --db-user <token> <db name>`
  * `tsh proxy db --tunnel --db-user <token> <db name>`
