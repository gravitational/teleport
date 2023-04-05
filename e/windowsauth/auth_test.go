package main

import "testing"

func TestLSA(t *testing.T) {
	setupDispatchTable()
	for name, test := range map[string]func(*testing.T){
		"toLSAString": testToLSAString,
	} {
		t.Run(name, test)
	}
}
