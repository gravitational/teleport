* https://github.com/lestrrat/go-libxml2
* https://github.com/onelogin/python-saml
   - reads settings from a JSON file (yuck)
   - BSD License (3-clause)
* TODO: understand xml bomb (https://pypi.python.org/pypi/defusedxml)

* Providers for SAML SP & IDP endpoints 
* Methods for generating and authenticating various SAML flows




Current working demo:

term1: go run ./example/idp/idp.go -bind :8001
term2: ngrok http 8001
term4: ngrok http 8000
edit example.go and fill in values for baseURL (term4) and idpMetadataURL (term2)
term3: go run ./example/example.go -bind :8000
term5: curl -v https://$SP.ngrok.io/saml/metadata | curl -v -H "Content-type: text/xml" --data-binary @- https://$IDP.ngrok.io/register-sp
browser: https://$SP.ngrok.io
