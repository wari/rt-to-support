package main

// Simplistic worker that just runs a download and loops forever until jobs are
// closed
func worker(id int, jobs <-chan request, result chan<- bool) {
	for j := range jobs {
		log.Debugln("Worker", id, "working on", j.ID)
		downloadFile(j.Client, j.Rt, j.ID, j.Filename)
		log.Debugln("Worker", id, "done on", j.ID)
		result <- true
	}
}
