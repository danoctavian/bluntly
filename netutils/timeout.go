package netutils

import (
	"errors"
	"time"
  "fmt"
)

/* channel timeout */

// reads 1 element from the channel with a timeout.
// unfortunately this piece of code is pretty useless in it's generalized form
//you can use a chan X as a chan interface{}, you need another chan to wrap it -_-
func ReadWithTimeout(ch <-chan (interface{}),
                     timeoutMillis time.Duration) (val interface{}, err error) {
  timeout := make(chan bool, 1) 
  go func() {
    time.Sleep(timeoutMillis * time.Millisecond)
    timeout <- true
  }()

  select {
    case v := <-ch:
    	val = v
    case <-timeout:
    	// the read from ch has timed out
    	err = errors.New(fmt.Sprintf("timed out on chan read from channel after %i millis.", timeoutMillis))
  }
  return 
}