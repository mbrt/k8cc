package controller

// Controller is an interface for controllers, allowing them to be run
type Controller interface {
	// Run starts the controller and takes ownership of the calling goroutine
	Run(threadiness int, stopCh <-chan struct{}) error
}
