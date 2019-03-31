package water

import (
	"testing"
)

const BUFFERSIZE = 1522

func startRead(t *testing.T, ifce *Interface, dataCh chan<- []byte, errCh chan<- error) {
	go func() {
		for {
			buffer := make([]byte, BUFFERSIZE)
			n, err := ifce.Read(buffer)
			if err != nil {
				errCh <- err
			} else {
				buffer = buffer[:n:n]
				dataCh <- buffer
			}
		}
	}()
}
