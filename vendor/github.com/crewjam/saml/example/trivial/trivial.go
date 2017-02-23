package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/crewjam/saml/samlsp"
)

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.Header.Get("X-Saml-Cn"))
}

func main() {
	key, _ := ioutil.ReadFile("myservice.key")
	cert, _ := ioutil.ReadFile("myservice.cert")
	samlSP, _ := samlsp.New(samlsp.Options{
		IDPMetadataURL: "https://www.testshib.org/metadata/testshib-providers.xml",
		URL:            "http://localhost:8000",
		Key:            string(key),
		Certificate:    string(cert),
	})
	app := http.HandlerFunc(hello)
	http.Handle("/hello", samlSP.RequireAccount(app))
	http.Handle("/saml/", samlSP)
	http.ListenAndServe(":8000", nil)
}
