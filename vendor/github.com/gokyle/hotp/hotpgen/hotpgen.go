package main

import (
	"encoding/base32"
	"flag"
	"fmt"
	"github.com/gokyle/hotp"
	"io/ioutil"
)

func main() {
	digits := flag.Int("d", 6, "number of digits")
	doRand := flag.Bool("r", false, "randomise counter")
	flag.Parse()

	var label string
	if flag.NArg() == 1 {
		label = flag.Arg(0)
	}

	otp, err := hotp.GenerateHOTP(*digits, *doRand)
	if err != nil {
		fmt.Printf("! %v\n", err.Error())
		return
	}

	url := otp.URL(label)
	png, err := otp.QR(label)
	if err != nil {
		fmt.Printf("! %v\n", err.Error())
		return
	}

	filename := label
	if label == "" {
		filename = base32.StdEncoding.EncodeToString([]byte(url))
	}
	err = ioutil.WriteFile(filename+".png", png, 0644)
	if err != nil {
		fmt.Printf("! %v\n", err.Error())
		return
	}

	err = ioutil.WriteFile(filename+".txt", []byte(url), 0644)
	if err != nil {
		fmt.Printf("! %v\n", err.Error())
		return
	}

	keyFile, err := hotp.Marshal(otp)
	if err != nil {
		fmt.Printf("! %v\n", err.Error())
		return
	}
	err = ioutil.WriteFile(filename+".key", keyFile, 0644)
	if err != nil {
		fmt.Printf("! %v\n", err.Error())
		return
	}
}
