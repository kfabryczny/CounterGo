package engine

import (
	"errors"
	"sync/atomic"
	"time"
)

type CancellationToken struct {
	active bool
}

func (ct *CancellationToken) Cancel() {
	ct.active = true
}

func (ct *CancellationToken) IsCancellationRequested() bool {
	return ct.active
}

var searchTimeout = errors.New("search timeout")

type TimeControlStrategy func(main, inc, moves int) (softLimit, hardLimit int)

type TimeManagement struct {
	start                       time.Time
	softTime                    time.Duration
	nodes, softNodes, hardNodes int64
	ct                          *CancellationToken
	timer                       *time.Timer
}

func (tm *TimeManagement) Nodes() int64 {
	return tm.nodes
}

func (tm *TimeManagement) IncNodes() {
	atomic.AddInt64(&tm.nodes, 1)
}

func (tm *TimeManagement) ElapsedMilliseconds() int64 {
	return int64(time.Since(tm.start) / time.Millisecond)
}

func (tm *TimeManagement) PanicOnHardTimeout() {
	if tm.ct.IsCancellationRequested() ||
		(tm.hardNodes > 0 && tm.nodes >= tm.hardNodes) {
		panic(searchTimeout)
	}
}

func (tm *TimeManagement) IsSoftTimeout() bool {
	return (tm.softTime > 0 && time.Since(tm.start) >= tm.softTime) ||
		(tm.softNodes > 0 && tm.nodes >= tm.softNodes)
}

func (tm *TimeManagement) Close() {
	if t := tm.timer; t != nil {
		t.Stop()
	}
}

func NewTimeManagement(limits LimitsType, timeControlStrategy TimeControlStrategy,
	side bool, ct *CancellationToken) *TimeManagement {
	var start = time.Now()

	if timeControlStrategy == nil {
		timeControlStrategy = TimeControlBasic
	}

	if ct == nil {
		ct = &CancellationToken{}
	}

	var main, increment int
	if side {
		main, increment = limits.WhiteTime, limits.WhiteIncrement
	} else {
		main, increment = limits.BlackTime, limits.BlackIncrement
	}

	var softTime, hardTime, softNodes, hardNodes int
	if limits.MoveTime > 0 {
		hardTime = limits.MoveTime
	} else if limits.Nodes > 0 {
		hardNodes = limits.Nodes
	} else if main > 0 {
		var softLimit, hardLimit = timeControlStrategy(main, increment, limits.MovesToGo)
		if limits.IsNodeLimits {
			softNodes, hardNodes = softLimit, hardLimit
		} else {
			softTime, hardTime = softLimit, hardLimit
		}
	}

	var timer *time.Timer
	if hardTime > 0 {
		timer = time.AfterFunc(time.Duration(hardTime)*time.Millisecond, func() {
			ct.Cancel()
		})
	}
	return &TimeManagement{
		start:     start,
		timer:     timer,
		ct:        ct,
		hardNodes: int64(hardNodes),
		softNodes: int64(softNodes),
		softTime:  time.Duration(softTime) * time.Millisecond,
	}
}

func computeLimit(main, inc, moves int) int {
	return (main + inc*(moves-1)) / moves
}

func TimeControlBasic(main, inc, moves int) (softLimit, hardLimit int) {
	const (
		SoftMovesToGo   = 50
		HardMovesToGo   = 10
		LastMoveReserve = 300
		MoveReserve     = 20
	)

	if moves == 0 {
		moves = SoftMovesToGo
	}

	softLimit = computeLimit(main, inc, min(moves, SoftMovesToGo))
	hardLimit = computeLimit(main, inc, min(moves, HardMovesToGo))

	hardLimit -= MoveReserve
	hardLimit = min(hardLimit, main-LastMoveReserve)
	hardLimit = max(hardLimit, 1)

	return
}