package summarizer

type workerPool struct {
	sem chan struct{}
}

func newWorkerPool(workers int) *workerPool {
	return &workerPool{
		sem: make(chan struct{}, workers),
	}
}

func (wp *workerPool) acquire() {
	wp.sem <- struct{}{}
}

func (wp *workerPool) release() {
	<-wp.sem
}
