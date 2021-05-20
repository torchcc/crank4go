package router_socket

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/torchcc/crank4go-core/util"
)

const defaultChanCapacity = 200

// iterable chan to mock LinkedBlockingQueue
type IterableChan struct {
	ch             chan *RouterSocket
	aliveSocketSet *sync.Map // map[*RouterSocket]struct{}
	// the number of alive sockets
	length int32
}

//
func (c *IterableChan) AliveSocketSlice() []*RouterSocket {
	slice := make([]*RouterSocket, 0, 8)
	c.aliveSocketSet.Range(func(key, _ interface{}) bool {
		socket := key.(*RouterSocket)
		slice = append(slice, socket)
		return true
	})
	return slice
}

// indicates if there is element in c.ch
func (c *IterableChan) IsEmpty() bool {
	return len(c.ch) == 0

}

func (c *IterableChan) IsAliveSocketSetEmpty() bool {
	return c.LenAlive() == 0
}

// the total element (including dead socket) in c.ch
func (c *IterableChan) Len() int {
	return len(c.ch)
}

// return the len of the current LinkedBlockingQueue
func (c *IterableChan) LenAlive() int {
	return int(atomic.LoadInt32(&c.length))
}

func (c *IterableChan) incrementAliveSocketNumber() {
	atomic.AddInt32(&c.length, 1)
}

func (c *IterableChan) decrementAliveSocketNumber() {
	atomic.AddInt32(&c.length, -1)
}

// remove a given alive item from the LinkedBlockingQueue,
// if the given socket is nil,  it is a no-op
// if the given socket is not in the queue, close the socket
// bool: indicates if the alive socket set has changed or not after return
func (c *IterableChan) Remove(socket *RouterSocket) bool {
	if socket == nil {
		return false
	}
	if _, loaded := c.aliveSocketSet.LoadAndDelete(socket); loaded {
		c.decrementAliveSocketNumber()
		return true
	} else {
		return false
	}
}

// it is non-blocking
func (c *IterableChan) Offer(socket *RouterSocket) {
	if socket == nil {
		return
	}
	go func() {
		for {
			select {
			case c.ch <- socket:
				c.aliveSocketSet.Store(socket, struct{}{})
				c.incrementAliveSocketNumber()
				return
			case <-time.After(5 * time.Minute):
				util.LOG.Warningf("can not push socket into channel, the channel of blocking queue is full, trying again in 5 min")
			}
		}
	}()
}

// it is non-blocking, return the first VALID item of a LinkedBlockingQueue, it the queue is empty, return nil
func (c *IterableChan) Poll() (socket *RouterSocket) {
	// do not delete for loop
	for {
		select {
		case socket = <-c.ch:
			if _, loaded := c.aliveSocketSet.LoadAndDelete(socket); loaded {
				c.decrementAliveSocketNumber()
				return socket
			} else { // is dead socket
				util.LOG.Warningf("Poll: got a Dead socket from channel, dropping it and going to poll again")
			}
		default:
			return nil
		}
	}
}

// if there is no alive socket util timeout, nil will be returned
func (c *IterableChan) PollTimeout(timeout time.Duration) (socket *RouterSocket) {
	begin := time.Now()

LOOP:
	select {
	case socket = <-c.ch:
		if _, loaded := c.aliveSocketSet.LoadAndDelete(socket); loaded {
			c.decrementAliveSocketNumber()
			util.LOG.Debugf("polled a socket from channel, socket: %s", socket.String())
			return socket
		}
	case <-time.After(timeout):
		util.LOG.Warningf("PollTimeout: case timeout")
		return nil
	}
	// is dead socket
	if time.Now().Sub(begin) > timeout {
		util.LOG.Warningf("PollTimeout, time out!: returning nil")
		return nil
	}
	util.LOG.Infof("got a dead socket, going to poll again...")
	goto LOOP
}

func (c *IterableChan) Range(f func(value interface{}) bool) {
	c.aliveSocketSet.Range(func(key, _ interface{}) bool {
		return f(key)

	})
}

// capacity for alive sockets and dead sockets
func NewIterableChan(capacity int) *IterableChan {
	if capacity < 0 {
		panic(errors.New("IllegalArgumentError: capacity should not be negative"))
	}
	if capacity == 0 {
		capacity = defaultChanCapacity
	}
	return &IterableChan{
		ch:             make(chan *RouterSocket, capacity),
		aliveSocketSet: &sync.Map{},
	}
}
