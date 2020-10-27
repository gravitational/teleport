
# `tsh` Bench


## Using `tsh` bench 

Specify the rps and duration with `--rate` and `--duration`. By default rate is 10 requests per seconds and duration is 1s. You will need to provide `[user@]host` and a command to run. 

Each request will be run concurrently in its own goroutine. 

```
# generate requests at 100 requests per second
# for 30 seconds
tsh bench --rate=100 --duration=30s localhost ls -l /
```


## Exporting Latency Profile 

Run `tsh bench` as usual, but use the `--export` flag to export. Use `--path` to specify a directory to save the file to. If the path is not specified, the file will be saved to your current working directory. 

Example:  

`tsh bench --export --rate 100 --duration 10s localhost ls -l /` 

The file its saved in the following format: latency_profile_2006-10-27_15:04:05.txt

### Plot the Profile 
1. Navigate to [HDR Histogram Plotter](http://hdrhistogram.github.io/HdrHistogram/plotFiles.html)
2. In the upper left corner of the page you will see a button that reads "choose files", click to choose the exported file

Your histogram should now be plotted! 


## Linear Benchmark Generator

### What is this?
A linear generator generates benchmarks between a lower and upper bound using a fixed step as configured by the user. 


### Use case
Linear generators are useful when benchmarking setups with understood performance profiles or generating graphs for user-facing materials.

Example: 

The following defined linear instance has a lower bound of 10 and upper bound of 50. This means the generator will run 5 benchmarks, starting with 10rps until 50rps incrementing each generation with the specified `Step`, which is 10 in this case. Each benchmark will run until, `MinimumWindow` and the `MinimumMeasurements` have been reached. 

```
	linear := &benchmark.Linear{
		LowerBound:          10,
		UpperBound:          50,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       30 * time.Second,
	}

```

_To see a full script example, go to `example.go` in `examples/bench`_