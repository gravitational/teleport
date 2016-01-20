package main

import (
	"flag"
	"fmt"
	"github.com/gokyle/hotp"
	"io/ioutil"
)

func main() {
	check := flag.Bool("c", false, "do integrity check")
	noUpdate := flag.Bool("n", false, "don't update counter")
	keyFile := flag.String("k", "hotp.key", "key file")
	url := flag.String("u", "", "URL to load new key from")
	write := flag.Bool("w", false, "only write URL-loaded key to file")
	flag.Parse()

	var otp *hotp.HOTP
	if *url != "" {
		var err error
		otp, _, err = hotp.FromURL(*url)
		if err != nil {
			fmt.Printf("[!] %v\n", err.Error())
			return
		}

		if *write {
			out, err := hotp.Marshal(otp)
			if err != nil {
				fmt.Printf("[!] %v\n", err.Error())
				return
			}

			err = ioutil.WriteFile(*keyFile, out, 0600)
			if err != nil {
				fmt.Printf("[!] %v\n", err.Error())
				return
			}

			return
		}
	} else {
		in, err := ioutil.ReadFile(*keyFile)
		if err != nil {
			fmt.Printf("[!] %v\n", err.Error())
			return
		}

		otp, err = hotp.Unmarshal(in)
		if err != nil {
			fmt.Printf("[!] %v\n", err.Error())
			return
		}
	}

	if *check {
		code, counter := otp.IntegrityCheck()
		fmt.Println("   code:", code)
		fmt.Println("counter:", counter)
	} else {
		fmt.Println(otp.OTP())
	}

	if !*noUpdate {
		out, err := hotp.Marshal(otp)
		if err != nil {
			fmt.Printf("[!] %v\n", err.Error())
			return
		}

		err = ioutil.WriteFile(*keyFile, out, 0600)
		if err != nil {
			fmt.Printf("[!] %v\n", err.Error())
			return
		}
	}
}
