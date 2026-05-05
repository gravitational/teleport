package main

import (
	"fmt"

	__antithesis_instrumentation__ "antithesis.notifier/ze55fecd173b5"
	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/random"
)

//line main.go:10
type Details map[string]any

//line main.go:12
func times10(x int) int {
	__antithesis_instrumentation__.Notify(1)

//line main.go:13
	assert.Sometimes(x%2 == 1, "input is sometimes odd", Details{"input": x})

//line main.go:14
	result := x * 10

//line main.go:15
	assert.Always(result%2 == 0, "result is always even", Details{"result": result})

//line main.go:16
	return result
}

//line main.go:19
func main() {
	__antithesis_instrumentation__.Notify(2)

//line main.go:20
	fmt.Println("Hello, world!")

//line main.go:21
	for i := 0; i < 50; i++ {
		__antithesis_instrumentation__.Notify(3)

//line main.go:22
		x := int(random.GetRandom() % 500)

//line main.go:23
		fmt.Printf("%v x 10 = %v\n", x, times10(x))
	}
}
