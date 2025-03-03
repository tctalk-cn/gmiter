package hotspot

import (
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/fatih/structs"
	jsoniter "github.com/json-iterator/go"
	"github.com/liuhailove/gmiter/core/base"
	"github.com/liuhailove/gmiter/core/hotspot/cache"
	"github.com/liuhailove/gmiter/logging"
	"github.com/liuhailove/gmiter/util"
	"github.com/pkg/errors"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var (
	jsonTraffic = jsoniter.ConfigCompatibleWithStandardLibrary
)

var (
// // 拒绝策略的token分配度量指标
// resourceRejectHotspotFlowThresholdGauge = metric_exporter.NewGauge(
//
//	"resource_reject_hotspot_flow_threshold",
//	"Resource reject hotspot flow threshold",
//	[]string{"resource"})
//
// // 线性限流的token分配度量指标
// resourceThrottlingHotspotFlowThresholdGauge = metric_exporter.NewGauge(
//
//	"resource_reject_hotspot_flow_threshold",
//	"Resource reject hotspot flow threshold",
//	[]string{"resource"})
)

func init() {
	//metric_exporter.Register(resourceRejectHotspotFlowThresholdGauge)
	//metric_exporter.Register(resourceThrottlingHotspotFlowThresholdGauge)
}

type TrafficShapingController interface {
	PerformChecking(arg interface{}, batchCount int64) *base.TokenResult

	BoundParamIndex() int

	ExtractArgs(ctx *base.EntryContext) interface{}

	BoundMetric() *ParamsMetric

	BoundRule() *Rule
}

type baseTrafficShapingController struct {
	r *Rule

	res           string
	metricType    MetricType
	paramIndex    int
	paramKey      string
	paramKind     ParamKind
	paramSource   ParameterSourceType
	threshold     float64
	specificItems map[interface{}]int64
	durationInSec int64
	metric        *ParamsMetric
}

func newBaseTrafficShapingControllerWithMetric(r *Rule, metric *ParamsMetric) *baseTrafficShapingController {
	if r.SpecificItems == nil {
		r.SpecificItems = make(map[interface{}]int64)
	}

	return &baseTrafficShapingController{
		r:             r,
		res:           r.Resource,
		metricType:    r.MetricType,
		paramIndex:    r.ParamIdx,
		paramKey:      r.ParamKey,
		paramKind:     r.ParamKind,
		paramSource:   r.ParamSource,
		threshold:     r.Threshold,
		specificItems: r.SpecificItems,
		durationInSec: r.DurationInSec,
		metric:        metric,
	}
}

func newBaseTrafficShapingController(r *Rule) *baseTrafficShapingController {
	switch r.MetricType {
	case QPS:
		size := 0
		if r.ParamsMaxCapacity > 0 {
			size = int(r.ParamsMaxCapacity)
		} else if r.DurationInSec == 0 {
			size = ParamsMaxCapacity
		} else {
			size = int(math.Min(float64(ParamsMaxCapacity), float64(ParamsCapacityBase*r.DurationInSec)))
		}
		if size <= 0 {
			logging.Warn("[HotSpot newBaseTrafficShapingController] Invalid size of cache, so use default value for ParamsMaxCapacity and ParamsCapacityBase",
				"ParamsMaxCapacity", ParamsMaxCapacity, "ParamsCapacityBase", ParamsCapacityBase)
			size = ParamsMaxCapacity
		}
		metric := &ParamsMetric{
			RuleTimeCounter:  cache.NewLRUCacheMap(size),
			RuleTokenCounter: cache.NewLRUCacheMap(size),
		}
		return newBaseTrafficShapingControllerWithMetric(r, metric)
	case Concurrency:
		size := 0
		if r.ParamsMaxCapacity > 0 {
			size = int(r.ParamsMaxCapacity)
		} else {
			size = ConcurrencyMaxCount
		}
		metric := &ParamsMetric{ConcurrentCounter: cache.NewLRUCacheMap(size)}
		return newBaseTrafficShapingControllerWithMetric(r, metric)
	default:
		logging.Error(errors.New("unsupported metric type"), "Ignoring the rule due to unsupported  metric type in Rule.newBaseTrafficShapingController()", "MetricType", r.MetricType.String())
		return nil
	}
}

func (c *baseTrafficShapingController) BoundMetric() *ParamsMetric {
	return c.metric
}

func (c *baseTrafficShapingController) performCheckingForConcurrencyMetric(arg interface{}) *base.TokenResult {
	specificItem := c.specificItems
	initConcurrency := new(int64)
	*initConcurrency = 0
	concurrencyPre := c.metric.ConcurrentCounter.AddIfAbsent(arg, initConcurrency)
	if concurrencyPre == nil {
		// First to access this arg
		return nil
	}
	concurrency := atomic.LoadInt64(concurrencyPre)
	concurrency++
	if specificConcurrency, existed := specificItem[arg]; existed {
		if concurrency <= specificConcurrency {
			return nil
		}
		msg := fmt.Sprintf("hotspot specific concurrency check blocked, arg: %v", arg)
		return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), concurrency)
	}
	threshold := c.threshold
	if concurrency <= int64(threshold) {
		return nil
	}
	msg := fmt.Sprintf("hotspot concurrency check blocked, arg: %v", arg)
	return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), concurrency)
}

// rejectTrafficShapingController use Reject strategy
type rejectTrafficShapingController struct {
	baseTrafficShapingController
	burstCount int64
}

// rejectTrafficShapingController use Throttling strategy
type throttlingTrafficShapingController struct {
	baseTrafficShapingController
	maxQueueingTimeMs int64
}

func (c *baseTrafficShapingController) BoundRule() *Rule {
	return c.r
}

func (c *baseTrafficShapingController) BoundParamKey() string {
	return c.paramKey
}

func (c *baseTrafficShapingController) BoundParamIndex() int {
	return c.paramIndex
}

// ExtractArgs 基于 TrafficShapingController 匹配来自 ctx 的 arg
// 如果匹配失败返回空.
func (c *baseTrafficShapingController) ExtractArgs(ctx *base.EntryContext) (value interface{}) {
	if c == nil {
		return nil
	}
	value = c.extractAttachmentArgs(ctx)
	if value != nil {
		return
	}
	value = c.extractHeader(ctx)
	if value != nil {
		return
	}
	value = c.extractMetadata(ctx)
	if value != nil {
		return
	}
	value = c.extractArgs(ctx)
	if value != nil {
		return
	}
	return
}

// extractHeader 从header中抽取参数
func (c *baseTrafficShapingController) extractHeader(ctx *base.EntryContext) interface{} {
	if ParameterTypeHeader != c.paramSource {
		return nil
	}
	headers := ctx.Input.Headers
	if headers == nil || len(headers) == 0 {
		return nil
	}
	vals := headers[c.paramKey]
	if len(vals) == 0 || len(vals[0]) == 0 {
		return nil
	}
	return transKind(vals[0], c.paramKind)
}

// extractMetadata 从Metadata中抽取参数
func (c *baseTrafficShapingController) extractMetadata(ctx *base.EntryContext) interface{} {
	if ParameterTypeMetadata != c.paramSource {
		return nil
	}
	metaData := ctx.Input.MetaData
	if metaData == nil || len(metaData) == 0 {
		return nil
	}
	val := metaData[c.paramKey]
	if len(val) == 0 {
		return nil
	}
	return transKind(val, c.paramKind)
}

// transKind 类型转换
func transKind(val string, kind ParamKind) interface{} {
	if kind == KindString {
		return val
	}
	var v interface{}
	var err error
	if kind == KindInt {
		v, err = strconv.Atoi(val)
	} else if kind == KindInt32 {
		vI64, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			v = int32(vI64)
		}
	} else if kind == KindInt64 {
		v, err = strconv.ParseInt(val, 10, 64)
	} else if kind == KindFloat32 {
		vF64, err := strconv.ParseFloat(val, 10)
		if err == nil {
			v = float32(vF64)
		}
	} else if kind == KindFloat64 {
		v, err = strconv.ParseFloat(val, 10)
	} else if kind == KindBool {
		v, err = strconv.ParseBool(val)
	}
	if err == nil {
		return v
	}
	return nil
}
func (c *baseTrafficShapingController) extractArgs(ctx *base.EntryContext) interface{} {
	args := ctx.Input.Args
	// 判断是否为结构体
	if structs.IsStruct(args[0]) {
		argsJsonData, _ := jsonTraffic.Marshal(args[0])
		var findObj, dataType, _, err = jsonparser.Get(argsJsonData, strings.Split(c.paramKey, ".")...)
		if err != nil {
			return nil
		}
		if dataType == jsonparser.Boolean {
			dataBool, _ := jsonparser.GetBoolean(argsJsonData, strings.Split(c.paramKey, ".")...)
			return dataBool
		} else if dataType == jsonparser.String {
			return string(findObj)
		} else if dataType == jsonparser.Number {
			dataNumber, _ := jsonparser.GetFloat(argsJsonData, strings.Split(c.paramKey, ".")...)
			return dataNumber
		}
		// 其他所有类型都转换为string
		return string(findObj)
	} else {
		// 判断是否为key/value
		for _, arg := range args {
			if argS, ok := arg.(string); ok {
				kv := strings.SplitN(argS, "=", 2)
				if len(kv) != 2 {
					continue
				}
				if c.paramKey == kv[0] {
					return kv[1]
				}
			}
		}
	}
	// 使用索引
	idx := c.BoundParamIndex()
	if idx < 0 {
		idx = len(args) + idx
	}
	if idx < 0 {
		if logging.DebugEnabled() {
			logging.Debug("[extractArgs] The param index of hotspot traffic shaping controller is invalid",
				"args", args, "paramIndex", c.BoundParamIndex())
		}
		return nil
	}
	if idx >= len(args) {
		if logging.DebugEnabled() {
			logging.Debug("[extractArgs] The argument in index doesn't exist",
				"args", args, "paramIndex", c.BoundParamIndex())
		}
		return nil
	}
	return args[idx]
}

func (c *baseTrafficShapingController) extractAttachmentArgs(ctx *base.EntryContext) interface{} {
	attachments := ctx.Input.Attachments
	if attachments == nil {
		if logging.DebugEnabled() {
			logging.Debug("[paramKey] The attachments of ctx is nil",
				"args", attachments, "paramKey", c.paramKey)
		}
		return nil
	}
	if c.paramKey == "" {
		if logging.DebugEnabled() {
			logging.Debug("[paramKey] The param key is nil",
				"args", attachments, "paramKey", c.paramKey)
		}
		return nil
	}
	arg, ok := attachments[c.paramKey]
	if !ok {
		if logging.DebugEnabled() {
			logging.Debug("[paramKey] extracted data does not exist",
				"args", attachments, "paramKey", c.paramKey)
		}
	}
	return arg
}

func (c *rejectTrafficShapingController) PerformChecking(arg interface{}, batchCount int64) *base.TokenResult {
	metric := c.metric
	if metric == nil {
		return nil
	}
	if c.metricType == Concurrency {
		return c.performCheckingForConcurrencyMetric(arg)
	} else if c.metricType > QPS {
		return nil
	}
	timeCounter := metric.RuleTimeCounter
	tokenCounter := metric.RuleTokenCounter
	if timeCounter == nil || tokenCounter == nil {
		return nil
	}

	// 计算可用token
	tokenCount := int64(c.threshold)
	val, existed := c.specificItems[arg]
	if existed {
		tokenCount = val
	}
	if tokenCount <= 0 {
		msg := fmt.Sprintf("hotspot reject check blocked, threshold is <= 0, arg: %v", arg)
		return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), nil)
	}

	maxCount := tokenCount + c.burstCount
	if batchCount > maxCount {
		// 返回被阻止，因为批次数量超过了rejectTrafficShapingController的最大计数
		msg := fmt.Sprintf("hotspot reject check blocked, request batch count is more than max token count, arg: %v", arg)
		return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), nil)
	}

	for {
		currentTimeInMs := int64(util.CurrentTimeMillis())
		lastAddTokenTimePre := timeCounter.AddIfAbsent(arg, &currentTimeInMs)
		if lastAddTokenTimePre == nil {
			// 首先填充token，并立即消耗token
			leftCount := maxCount - batchCount
			tokenCounter.AddIfAbsent(arg, &leftCount)
			//resourceRejectHotspotFlowThresholdGauge.Set(float64(leftCount), c.BoundRule().Resource)
			return nil
		}
		// 计算自添加最后一个令牌以来的持续时间。
		passTime := currentTimeInMs - atomic.LoadInt64(lastAddTokenTimePre)
		if passTime > c.durationInSec*1000 {
			// 由于统计窗口已过，请重新填充令牌
			leftCount := maxCount - batchCount
			oldQpsPtr := tokenCounter.AddIfAbsent(arg, &leftCount)
			if oldQpsPtr == nil {
				// 这里可能不准确
				atomic.StoreInt64(lastAddTokenTimePre, currentTimeInMs)
				return nil
			}
			// 重新填充token
			restQps := atomic.LoadInt64(oldQpsPtr)
			toAddTokenNum := passTime * tokenCount / (c.durationInSec * 1000)
			newQps := int64(0)
			if toAddTokenNum+restQps > maxCount {
				newQps = maxCount - batchCount
			} else {
				newQps = toAddTokenNum + restQps - batchCount
			}
			if newQps < 0 {
				msg := fmt.Sprintf("hotspot reject check blocked, request batch count is more than available token count, arg: %v", arg)
				return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), nil)
			}
			if atomic.CompareAndSwapInt64(oldQpsPtr, restQps, newQps) {
				atomic.StoreInt64(lastAddTokenTimePre, currentTimeInMs)
				//resourceRejectHotspotFlowThresholdGauge.Set(float64(restQps), c.BoundRule().Resource)
				return nil
			}
			runtime.Gosched()
		} else {
			// 检查剩余的 token 是否足以批处理
			oldQpsPtr, found := tokenCounter.Get(arg)
			if found {
				oldRestToken := atomic.LoadInt64(oldQpsPtr)
				if oldRestToken-batchCount >= 0 {
					//update
					if atomic.CompareAndSwapInt64(oldQpsPtr, oldRestToken, oldRestToken-batchCount) {
						//resourceRejectHotspotFlowThresholdGauge.Set(float64(oldRestToken-batchCount), c.BoundRule().Resource)
						return nil
					}
				} else {
					msg := fmt.Sprintf("hotspot reject check blocked, request batch count is more than available token count, arg: %v", arg)
					return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), nil)
				}
			}
			runtime.Gosched()
		}
	}
}

func (c *throttlingTrafficShapingController) PerformChecking(arg interface{}, batchCount int64) *base.TokenResult {
	metric := c.metric
	if metric == nil {
		return nil
	}
	if c.metricType == Concurrency {
		return c.performCheckingForConcurrencyMetric(arg)
	} else if c.metricType > QPS {
		return nil
	}

	timeCounter := metric.RuleTimeCounter
	tokenCounter := metric.RuleTokenCounter
	if timeCounter == nil || tokenCounter == nil {
		return nil
	}
	// calculate available token
	tokenCount := int64(c.threshold)
	val, existed := c.specificItems[arg]
	if existed {
		tokenCount = val
	}
	if tokenCount <= 0 {
		msg := fmt.Sprintf("hotspot throttling check blocked, threshold is <= 0, arg: %v", arg)
		return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), nil)
	}
	intervalCostTime := int64(math.Round(float64(batchCount * c.durationInSec * 1000 / tokenCount)))
	for {
		currentTimeInMs := int64(util.CurrentTimeMillis())
		lastPassTimePtr := timeCounter.AddIfAbsent(arg, &currentTimeInMs)
		if lastPassTimePtr == nil {
			// first access arg
			return nil
		}
		// load the last pass time
		lastPassTime := atomic.LoadInt64(lastPassTimePtr)
		// calculate the expected pass time
		expectedTime := lastPassTime + intervalCostTime
		if expectedTime <= currentTimeInMs || expectedTime-currentTimeInMs < c.maxQueueingTimeMs {
			if atomic.CompareAndSwapInt64(lastPassTimePtr, lastPassTime, currentTimeInMs) {
				awaitTime := expectedTime - currentTimeInMs
				if awaitTime > 0 {
					atomic.StoreInt64(lastPassTimePtr, expectedTime)
					return base.NewTokenResultShouldWait(time.Duration(awaitTime) * time.Millisecond)
				}
				return nil
			} else {
				runtime.Gosched()
			}
		} else {
			msg := fmt.Sprintf("hotspot throttling check blocked,wait time exceedes max queueing time, arg: %v", arg)
			return base.NewTokenResultBlockedWithCause(base.BlockTypeHotSpotParamFlow, msg, c.BoundRule(), nil)
		}
	}
}
