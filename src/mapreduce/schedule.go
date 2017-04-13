package mapreduce

import "fmt"
import "sync"

// schedule starts and waits for all tasks in the given phase (Map or Reduce).
func (mr *Master) schedule(phase jobPhase) {
	var ntasks int
	var nios int // number of inputs (for reduce) or outputs (for map)
	switch phase {
	case mapPhase:
		ntasks = len(mr.files)
		nios = mr.nReduce
	case reducePhase:
		ntasks = mr.nReduce
		nios = len(mr.files)
	}

	fmt.Printf("Schedule: %v %v tasks (%d I/Os)\n", ntasks, phase, nios)

	// All ntasks tasks have to be scheduled on workers, and only once all of
	// them have been completed successfully should the function return.
	// Remember that workers may fail, and that any given worker may finish
	// multiple tasks.
	//
	// TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO
	//

	var waitGroup sync.WaitGroup

	for i := 0; i < ntasks; i++ {
		waitGroup.Add(1)
		go func(task_no int, nios int, phase jobPhase) {
			debug("DEBUG: current taskNum: %v, nios: %v, phase: %v\n", task_no, nios, phase)
			defer waitGroup.Done()

			for {
				worker := <-mr.registerChannel
				debug("DEBUG: current worker port: %v \n", worker)

				var args DoTaskArgs

				args.JobName = mr.jobName
				args.Phase = phase
				args.File = mr.files[task_no]
				args.NumOtherPhase = nios
				args.TaskNumber = task_no

				ok := call(worker, "Worker.DoTask", &args, new(struct{}))

				if ok {
					go func() {
						mr.registerChannel <- worker
					}()
					break
				}

			}
		}(i, nios, phase)

	}

	waitGroup.Wait()

	fmt.Printf("Schedule: %v phase done\n", phase)
}
