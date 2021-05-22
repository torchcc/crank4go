package darklaunch_manager

import (
	"sync/atomic"

	"github.com/torchcc/crank4go/util"
)

var toggle int32

func TurnGrayTestingOn(req string) {
	util.LOG.Infof("turn gray testing on as user request %s", req)
	atomic.StoreInt32(&toggle, 1)
}

func TurnGrayTestingOff(req string) {
	util.LOG.Infof("turn gray testing off as user request %s", req)
	atomic.StoreInt32(&toggle, 0)
}

func IsGrayTestingOn() bool {
	return toggle == 1
}
