package testutil

var intSource <-chan int

func init() {
	intSource = func() <-chan int {
		out := make(chan int, 25)
		go func() {
			id := 0
			for {
				out <- id
				id++
			}
		}()
		return out
	}()
}

// GetPortNumber returns a new port number that has not been used in the
// current runtime.
func GetPortNumber(base int) int {
	return base + <-intSource
}
