package flow

import (
	"github.com/liuhailove/gmiter/core/base"
	"github.com/liuhailove/gmiter/util"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestThrottlingChecker_DoCheck(t *testing.T) {
	util.SetClock(util.NewMockClock())

	intervalMs := 10000
	threshold := 50.0
	timeoutMs := 0

	tc := NewThrottlingChecker(nil, uint32(timeoutMs), uint32(intervalMs))

	// Should block when batchCount > threshold.
	res := tc.DoCheck(nil, uint32(threshold+1.0), threshold)
	assert.True(t, res != nil && res.IsBlocked())

	// The first request will pass.
	res = tc.DoCheck(nil, uint32(threshold), threshold)
	assert.True(t, res == nil || res.IsPass())

	reqCount := 10
	for i := 0; i < reqCount; i++ {
		assert.True(t, tc.DoCheck(nil, 1, threshold).IsBlocked())
	}
	util.Sleep(time.Duration(intervalMs/int(threshold)*reqCount+10) * time.Millisecond)

	assert.True(t, tc.DoCheck(nil, 1, threshold) == nil)
	assert.True(t, tc.DoCheck(nil, 1, threshold).IsBlocked())
}

func TestThrottlingChecker_DoCheckSingleThread(t *testing.T) {
	util.SetClock(util.NewMockClock())
	intervalMs := 10000
	threshold := 50.0
	timeoutMs := 2000

	tc := NewThrottlingChecker(nil, uint32(timeoutMs), uint32(intervalMs))

	// Should block when batchCount > threshold.
	res := tc.DoCheck(nil, uint32(threshold+1.0), threshold)
	assert.True(t, res != nil && res.IsBlocked())

	// The first request will pass.
	res = tc.DoCheck(nil, uint32(threshold), threshold)
	assert.True(t, res == nil || res.IsPass())

	resultList := make([]*base.TokenResult, 0)
	reqCount := 20
	for i := 0; i < reqCount; i++ {
		res := tc.DoCheck(nil, 1, threshold)
		resultList = append(resultList, res)
	}

	// waitCount is count of request that will wait and not be blocked
	waitCount := int(float64(timeoutMs) / (float64(intervalMs) / threshold))
	for i := 0; i < waitCount; i++ {
		assert.True(t, resultList[i].Status() == base.ResultStatusShouldWait)
		wt := resultList[i].NanosToWait()
		assert.InEpsilon(t, (i+1)*(int)(time.Second/time.Nanosecond)/waitCount, wt, 10)
	}
	for i := waitCount; i < reqCount; i++ {
		assert.True(t, resultList[i].IsBlocked())
	}
}

func TestThrottlingChecker_DoCheckQueueingParallel(t *testing.T) {
	intervalMs := 10000
	threshold := 50.0
	timeoutMs := 0

	tc := NewThrottlingChecker(nil, uint32(timeoutMs), uint32(intervalMs))

	assert.True(t, tc.DoCheck(nil, 1, threshold) == nil)

	wg := &sync.WaitGroup{}
	gc := 24
	wg.Add(gc)

	var waitCount, blockCount int32 = 0, 0
	for i := 0; i < gc; i++ {
		go func() {
			res := tc.DoCheck(nil, 1, threshold)
			if res.IsBlocked() {
				atomic.AddInt32(&blockCount, 1)
			}
			if res.Status() == base.ResultStatusShouldWait {
				atomic.AddInt32(&waitCount, 1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(gc), waitCount+blockCount)
	// Non-strict mode may not be strictly accurate, so here we tolerate a delta.
	assert.InEpsilon(t, threshold/(float64(intervalMs)/1000.0), waitCount, 1)
}

func TestThrottlingChecker_DoCheckParallelPass(t *testing.T) {
	oldClock := util.CurrentClock()
	util.SetClock(util.NewMockClock())
	defer util.SetClock(oldClock)

	intervalMs := 10000
	threshold := 50.0
	timeoutMs := 0

	tc := NewThrottlingChecker(nil, uint32(timeoutMs), uint32(intervalMs))

	wg := &sync.WaitGroup{}
	beginWg := &sync.WaitGroup{}
	gc := 512
	wg.Add(gc)
	beginWg.Add(gc)

	var passCount int32 = 0
	for i := 0; i < gc; i++ {
		go func() {
			// Preheating
			beginWg.Done()
			beginWg.Wait()

			defer wg.Done()
			res := tc.DoCheck(nil, 1, threshold)
			if res == nil {
				atomic.AddInt32(&passCount, 1)
				return
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), passCount)
}
