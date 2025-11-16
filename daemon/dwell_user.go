package daemon

import (
	"math"
	"sync"
)

type ioEvent struct {
	Pid          uint32
	BytesWritten uint64
	UniqueFiles  uint32
	TimestampNS  uint64
}

type tierID int

const (
	tierTrusted tierID = iota
	tierIntermediate
	tierUntrusted
)

type tierConfig struct {
	budget, omega1, omega2 float64
}

var tierConfigs = map[tierID]tierConfig{
	tierTrusted:      {budget: 12000, omega1: 0.9, omega2: 0.1},
	tierIntermediate: {budget: 8000, omega1: 0.55, omega2: 0.45},
	tierUntrusted:    {budget: 4000, omega1: 0.3, omega2: 0.7},
}

const wipAlpha = 0.6

type pidWIPState struct {
	price float64
	tier  tierID
}

var (
	pidStateMu sync.RWMutex
	pidStates  = make(map[uint32]*pidWIPState)
)

func classifyTier(tbwMB, ufm float64) tierID {
	switch {
	case tbwMB >= 10000 || ufm <= 1000:
		return tierTrusted
	case ufm >= 20000 && tbwMB >= 500:
		return tierIntermediate
	default:
		return tierUntrusted
	}
}

func (c *Controller) ensurePIDState(pid uint32) *pidWIPState {
	pidStateMu.Lock()
	defer pidStateMu.Unlock()
	state, ok := pidStates[pid]
	if !ok {
		state = &pidWIPState{price: 0}
		pidStates[pid] = state
	}
	return state
}

func (c *Controller) handleIOEvent(evt ioEvent) {
	tbwMB := float64(evt.BytesWritten) / (1024 * 1024)
	ufm := float64(evt.UniqueFiles)
	tier := classifyTier(tbwMB, ufm)
	cfg := tierConfigs[tier]
	wip := cfg.omega1*tbwMB + cfg.omega2*ufm

	state := c.ensurePIDState(evt.Pid)
	state.tier = tier
	state.price = math.Max(0, state.price+wipAlpha*(wip-cfg.budget))

	// ...existing metrics updates and enforcement hooks...
}
