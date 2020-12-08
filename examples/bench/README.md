# `tsh bench`

## Using `tsh` bench 
Specify the rps and duration with `--rate` and `--duration`. By default rate is 10 requests per seconds and duration is 1 second. You will need to provide `[user@]host` and a command to run. 

```
# generate requests at 100 requests per second
# for 30 seconds
tsh bench --rate=100 --duration=30s localhost ls -l /
```


## Exporting Latency Profile 
Run `tsh bench` as usual and use the `--export` flag to export the response histogram. Use `--path` to specify a directory to save the profile to. If the path is not specified, the file will be saved to your current working directory. 

Example:  

`tsh bench --export --rate 100 --duration 10s localhost ls -l /` 

The file will be saved in the following format:  
`latency_profile_2006-10-27_15:04:05.txt`


### Plot the Profiles 
1. Navigate to [HDR Histogram Plotter](http://hdrhistogram.github.io/HdrHistogram/plotFiles.html)
2. In the upper left corner of the page you will see a button that reads "choose files" and select the file 

Your histogram should now be plotted.

## Linear Benchmark Generator
A linear generator generates benchmarks between a lower and upper bound using a fixed step as configured by the user. 


### Use case
Linear generators are useful when benchmarking setups with understood performance profiles or generating graphs for user-facing materials.

Example: 

The following defined linear instance has a lower bound of 10 and upper bound of 50. This means the generator will run 5 benchmarks, starting with 10rps until 50rps incrementing each generation with the specified `Step`, which is 10rps in the example. Each benchmark will run until, `MinimumWindow` and the `MinimumMeasurements` have been reached. 

```
	linear := &benchmark.Linear{
		LowerBound:          10,
		UpperBound:          50,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       30 * time.Second,
	}

```

_Full script example, see `examples/bench/example.go`_