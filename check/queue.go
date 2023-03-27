package check

// Queue is used by a server.Server to feed it work (Checks to execute).
type Queue interface {
	Enqueue(chk *Check)
	Dequeue() *Check
	Count() uint64
}
