package launchers

type ParallelWorker struct {
	count   int
	results chan error
}

func (worker *ParallelWorker) Add(job func() error) {
	worker.count++
	if worker.results == nil {
		worker.results = make(chan error)
	}
	go worker.run(job)
}

func (worker *ParallelWorker) run(job func() error) {
	var err error
	func() {
		defer func() { err, _ = recover().(error) }()
		err = job()
	}()
	worker.results <- err
}

func (worker *ParallelWorker) Join() error {
	for worker.count > 0 {
		err := <-worker.results
		if err != nil {
			return err
		}
		worker.count--
	}
	return nil
}
