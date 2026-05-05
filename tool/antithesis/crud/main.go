package main

import (
	"fmt"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/random"
)

type Details map[string]any

func times10(x int) int {
	assert.Sometimes(x%2 == 1, "input is sometimes odd", Details{"input": x})
	result := x * 10
	assert.Always(result%2 == 0, "result is always even", Details{"result": result})
	return result
}

func main() {
	fmt.Println("Hello, world!")
	for i := 0; i < 50; i++ {
		x := int(random.GetRandom() % 500)
		fmt.Printf("%v x 10 = %v\n", x, times10(x))
	}
}
