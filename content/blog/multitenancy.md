+++
author = ["Sasha Klizhentas"]
categories = ["post"]
date = "2020-07-08"
tags = ["engineering"]
title = "Programming language runtimes are not ready for multitenant services"
description = "Programming language runtimes are not handling multi-tenant services well, so resillient SaaS has to be single tenant"
+++


Imagine a typical software as a service app, running a nodejs/php/python process
connected to a postgres database. It seems perfectly reasonable for this process
to handle requests for multiple customers at the same time. 

Let’s dig into common failures caused by this multitenant architecture,
and review possible alternative design choices we’ve used when building our
own SaaS - Teleport cloud.


## Language Runtimes and Multi Tenancy

Most programming languages implement runtime systems that help them to execute code.
Let’s take a simple Go “Saas” service that fetches the contents of a URL
for multiple tenants in parallel:

```go

package main

import (
    "fmt"
    "net/http"
    "sync"
)

// fetch in our example performs some SaaS work for a tenant,
// in this case it fetches the URL contents from the internet!
func work(wg *sync.WaitGroup, tenant int, url string) {
    defer wg.Done()
    // Don't ignore errors in real code, folks, and close the response body
    re, err := http.Get(url)
    if err != nil {
        fmt.Printf("Tenant(%v) error -> ->  %v\n", tenant, err)
        return
    }
    defer re.Body.Close()
    fmt.Printf("Tenant(%v) -> %v\n", tenant, re.Status)
}

func main() {
    // wg waits for a collection of goroutines to finish
    // https://golang.org/pkg/sync/#WaitGroup
    wg := sync.WaitGroup{}
    url := "http://www.golang.org/"
    for tenant := 0; tenant < 3; tenant++ {
        wg.Add(1)
        // go starts a goroutine, a lightweight thread managed by runtime
        // https://tour.golang.org/concurrency/1
        go work(&wg, tenant, url)
    }
    // wait for all goroutines to finish
    wg.Wait()
}

```

If you run this program, you get something like this executed really fast:

```bash
$ go run simple/main.go 
Tenant(2) -> 200 OK
Tenant(1) -> 200 OK
Tenant(0) -> 200 OK
```

To make this happen, we can imagine that Go runtime generates the following pseudo code:

```
goroutine0 = call work with args(0, "http://golang.org")
handle0 = http.Get("http://golang.org")
handles.Add(handle0, goroutine0)
sleepingGorotuines.Add(goroutine0)

goroutine0 = call work with args(1, "http://golang.org")
handle1 = http.Get("http://golang.org")
handles.Add(handle1, goroutine1)
sleepingGorotuines.Add(goroutine1)
...
// main program waits until any function returns data from the internet
handle = epoll(handles)
... 
// Continue execution of goroutine 0
print(response)
// main program waits until any other handle returns data from the internet
handle = epoll(handles)
// Continue execution of goroutine 0
print(response)
```

As you see, parallelism and “blocking” call here is an illusion created by a
combination of using asynchronous [epoll linux](https://kovyrin.net/2006/04/13/epoll-asynchronous-network-programming/)
function that can efficiently poll many handles at once and powerful Go runtime
that handles “go” function calls using a smart scheduler.

If you are interested in a more detailed intro to the Go scheduler, check out this article
from [awesome Adran labs](https://www.ardanlabs.com/blog/2018/08/scheduling-in-go-part2.html).

This by the way, makes Go a perfect candidate for handling concurrent network requests
that constitute a bulk of an average software as a service program.

So what’s the problem? Well, Go scheduler (and other similar schedulers) does not really handle
well situations like this one:

```go
package main

import (
    "fmt"
    "sync"
    "time"
)

func factorial(n int64) int64 {
    b := int64(1)
    for i := int64(1); i <= n; i++ {
        b *= i
    }
    return b
}

func work(wg *sync.WaitGroup, tenant int, number int64) {
    defer wg.Done()
    number = factorial(number)
    end := time.Now().UTC()
    fmt.Printf("Tenant(%v) -> %v at %v\n", tenant, number, end.Format("15:04:05.000000"))
}

func main() {
    // wg waits for a collection of goroutines to finish
    // https://golang.org/pkg/sync/#WaitGroup
    wg := sync.WaitGroup{}
    for tenant := 0; tenant < 10; tenant++ {
        wg.Add(1)
        // go starts a goroutine, a lightweight thread managed by runtime
        // https://tour.golang.org/concurrency/1
        if tenant == 0 {
            go work(&wg, tenant, 10000000000)
        } else {
            go work(&wg, tenant, 10+int64(tenant))
        }
    }
    // wait for all goroutines to finish
    wg.Wait()
}
```

In this case,  you would not observe the same parallelism. In my case, function
(and let’s ignore the problems with factorial implementation for a moment), will print something like this:

```bash
$ GOMAXPROCS=1 go run fib/main.go
Tenant(9) -> 121645100408832000 at 00:20:42.111520
Tenant(1) -> 39916800 at 00:20:52.480150
Tenant(2) -> 479001600 at 00:20:52.480171
Tenant(3) -> 6227020800 at 00:20:52.480174
Tenant(4) -> 87178291200 at 00:20:52.480177
Tenant(5) -> 1307674368000 at 00:20:52.480179
Tenant(6) -> 20922789888000 at 00:20:52.480181
Tenant(7) -> 355687428096000 at 00:20:52.480184
Tenant(8) -> 6402373705728000 at 00:20:52.480209
Tenant(0) -> 0 at 00:20:52.480212
```

Because of my problematic factorial implementation that took a lot of iterations
and number crunching, tenant 0 really consumed all CPU time and degraded performance for all other tenants.

We can of course implement an efficient (and correct) implementation of factorial function and even add
code that limits the possible input numbers (all great ideas in a real SaaS), but our service has just
suffered an outage due to a bug in the code for all our customers, pager duty is ringing and we are
not sleeping (again).

## Single Tenant Kubernetes as a Solution

The core of the problem is that CPU time is limited, and programming language runtime is not
capable of efficiently distributing the CPU time per tenant, it lacks context and control over the
distribution of the CPU time per goroutine. 

In a single tenant world, this is still a problem, however by assuming that only one tenant
uses a single process at a time, we have more controls over the isolation:

https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

