import React from 'react'
import { storiesOf } from '@storybook/react'
import { HubLicenses } from './HubLicenses'

storiesOf('GravityHub/HubLicenses', module)
  .add('HubLicenses', () => {
    return (
      <HubLicenses
        attempt={{ } }
        attemptActions={{}}
      />
    );
  })
  .add('With Licenses', () => {
    return (
      <HubLicenses
        license={license}
        attempt={{ } }
        attemptActions={{}}
      />
    );
  })
  .add('With Error', () => {
    return (
      <HubLicenses
        attempt={{
          isFailed: true,
          message: 'Server side error'
         } }
        attemptActions={{}}
      />
    );
  });


const license = `-----BEGIN CERTIFICATE-----
MIIDvDCCAqSgAwIBAgIUTHTVTmRA0DW71lFosqbiit/jH2cwDQYJKoZIhvcNAQEL
BQAwGzEZMBcGA1UEAxMQZ3Jhdml0YXRpb25hbC5pbzAeFw0xOTA0MjYwMjQzNDFa
Fw0xOTA0MjYxNjAwMDBaMC0xGTAXBgNVBAoTEGdyYXZpdGF0aW9uYWwuaW8xEDAO
BgNVBAMTB2xpY2Vuc2UwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCs
1VrqbjN6mX0zl9WyA8VqHnTBSEqjJr4Qc3hruZc1wWR/i16INlduZ9e8M2rYz+3P
U/ixhdubAsN6FGPaBYsEqOHd/0Fu/lPPCjb+BhVcSeUN4o5Vq+HaqLlvspDHJgod
V0RdfJVXagGn3aVbaPZSIuyo+J05TP180GsyYSZ5H4sjFmf4ukUoSnlOqL2PV9jP
CR0YxLXYt7+5okSE4i7nrwarlBUhlbbKzpKwVbllpVHudh2HGEQKt56Rc6rl7kJ4
wsJ26bD1jcufqlds2+pLbo65EQZfHw8N75/gpHFcezzy7/Kpp2+xIoGR01iGlKOq
a9AIujAxuIfEOwc6at0tAgMBAAGjgeUwgeIwDgYDVR0PAQH/BAQDAgWgMB0GA1Ud
JQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB0GA1UdDgQW
BBS4i8l2XC9nhsNOcOOm1RsqP9kx/DAfBgNVHSMEGDAWgBQpdv46W/pps04uxdNS
EsnZyKIpsDAPBgNVHREECDAGhwR/AAABMFIGAlUqBEx7ImV4cGlyYXRpb24iOiIy
MDE5LTA0LTI2VDE2OjAwOjAwLjEzOTUwMDI5WiIsIm1heF9ub2RlcyI6Mywic2h1
dGRvd24iOnRydWV9MA0GCSqGSIb3DQEBCwUAA4IBAQDDW3L2hBn1cgLaFDvtOM+R
qQ8gXeILhTqP1LwZYkDcpgPi1BeBjzQuOi2AMwb9zFwcYvZ1EL7XTtrZMlq9y3ny
gyiW6AxXd0XexmM98rFxxqw94OMkHk66zwnso/uquRvgwefqqWsMkJxLGLTmZfDp
6xB1lVqkNrvvGzz3xdMHmj1fqcDsauENPCLP/azy3HNo1q9XHsGHur//+GVlNMxf
Srz1MogqLA8a8afVOgB8Q8cX5ggdAtD7IBwHr9xeZblrqoahEpj5ZheDRzqdqBqi
ND7qpHoPIwxdLhFoEJNamsefn0C6ePLucpnt/V1FVe18RsNPVl3QiN3AcOqPgWSD
-----END CERTIFICATE-----
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEArNVa6m4zepl9M5fVsgPFah50wUhKoya+EHN4a7mXNcFkf4te
iDZXbmfXvDNq2M/tz1P4sYXbmwLDehRj2gWLBKjh3f9Bbv5Tzwo2/gYVXEnlDeKO
Vavh2qi5b7KQxyYKHVdEXXyVV2oBp92lW2j2UiLsqPidOUz9fNBrMmEmeR+LIxZn
+LpFKEp5Tqi9j1fYzwkdGMS12Le/uaJEhOIu568Gq5QVIZW2ys6SsFW5ZaVR7nYd
hxhECreekXOq5e5CeMLCdumw9Y3Ln6pXbNvqS26OuREGXx8PDe+f4KRxXHs88u/y
qadvsSKBkdNYhpSjqmvQCLowMbiHxDsHOmrdLQIDAQABAoIBAQCZWDdxFlOgbDyU
oRud9RCcBeercdufBAnQiNMIKUNLE4p6S4qVKjnKoGHd/nTHZzzlHejigRSGZR5Q
23R4hUCB4uF72TUSKJ7tbG+8VGNxXbLX7fJBet5J5jeZLgKcX1jMDZh/pcDPLSPI
77P99ZPO7mOxy9ubcn7Z1gW2TlIXVfsDvc7DG6X1v/032vQIrgVGb2t0ouJj3Fr6
2jVeux6ulAkNcwJ9TDCw5XdTIZuBDWOVQsY7dkqknSKJteyk4PetPpcHdnO1HCG0
PQ3m8OWG9DOMfla//PQSTkwmRKIYLJyvI9fSwr54uEEv1bfAqalyHaZqT3c0jlbu
P1OT2sa1AoGBANau5aB6zw+sMDOpLANmY0rGMxNn+7x9XiKn3neBL95fAGUJAwt9
J3f5tm5F5MWGfkQnfYQ6hI9RLN2JsA4SmnA+gqGic2CqqvG2qxBD4Rm10RD9mFWC
ml2thJi9pbCL6rMeiO0Q0P/QJaxeFLewALR7VxiXS1rQVsgx+9P4G8GXAoGBAM4Y
lfDXf1SzvStwMCuWMFahN46Dbpi9/OCNVA+dBUIm3rx3ihGNb+V7h0XH3YyzrC/L
JKmhwcCtp+c4LqgdXkVGuNRlQ0BKdtVY7kq/jP0vDFfGLdzUPiCPQ8/FZWUFR1T+
qTc0CEzc12/vzbjqHo0AcYIxgvkqOAScXg3+i+fbAoGAScvVI1UT2E9YQmnkt0Z6
2zlGVWVpI2H0+fS6hFnkGoyNli2C3nAnIRa1nzJncX7J6KOqgcmbx6gfxAeQfUXn
0K2sOeOdxZzlJjGkm/K5bh0RwMVrl/lNFuaOrfKDAi0WgHv+lX8yWL00NgwhEwNt
Op0rU0iunoj/S9HivvqKkAECgYBlBKICAe60msEfWIcT5jLdU3pCzWNZVM5tVnic
io94REsqv8EaJ2RwbCL67iNHAw5kAsN+rf2lLrk82UntNy/s7uRLnzLegWFL46Ix
W0CFHRmEsGvscM/e77oCTjQL1xGGtKhGmadz3U9v22/Pslm1LUF12kTjUnFQuUBU
xa7XvwKBgHeUpRJoUL56qbVkAFRN8ptSqaKonuFk7Gk7gYsuqlvW0Q+Nr6VOywpd
VzH+NOvugvLiGoXk2Cg2Vd60tDdaWWCT9OlDG9cSVCuMVwwHZ2mkfs/F2m/Ip4zK
pn7caID+Qlffl1TWaMbiyRYh122+DMILq49pK4mUm4PogNdphzRX
-----END RSA PRIVATE KEY-----
`