package circuitbreaker

import (
	"github.com/liuhailove/gmiter/core/base"
)

const (
	StatSlotOrder = 5000
)

var (
	DefaultMetricStatSlot = &MetricStatSlot{}
)

// MetricStatSlot records metrics for circuit breaker on invocation completed.
// MetricStatSlot must be filled into slot chain if circuit breaker is alive.
type MetricStatSlot struct {
}

func (m MetricStatSlot) Order() uint32 {
	return StatSlotOrder
}

// Initial
//
// 初始化，如果有初始化工作放入其中
func (m MetricStatSlot) Initial() {}

func (m MetricStatSlot) OnEntryPassed(ctx *base.EntryContext) {
	// Do nothing
	return
}

func (m MetricStatSlot) OnEntryBlocked(ctx *base.EntryContext, blockError *base.BlockError) {
	// Do nothing
	return
}

func (m MetricStatSlot) OnCompleted(ctx *base.EntryContext) {
	res := ctx.Resource.Name()
	err := ctx.Err()
	rt := ctx.Rt()
	for _, cb := range getBreakersOfResource(res) {
		cb.OnRequestComplete(rt, err)
	}
}
