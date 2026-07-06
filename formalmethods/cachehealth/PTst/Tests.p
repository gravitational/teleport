test tcBug[main = Driver]: assert MetricEventuallyConverges in {
    FaultInjector, Driver, MetricBug -> Metric, Cache };

test tcFix[main = Driver]: assert MetricEventuallyConverges in {
    FaultInjector, Driver, Metric, Cache };
