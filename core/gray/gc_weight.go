package gray

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/go-basic/uuid"
	"github.com/liuhailove/gmiter/core/base"
	"github.com/liuhailove/gmiter/logging"
	"math/rand"
	"strconv"
	"strings"
)

// WeightTrafficSelector 权重流量选择器
type WeightTrafficSelector struct {
	// owner 所归属的流量选择controller
	owner *TrafficSelectorController

	// resources 用于权重计算的资源对象
	resources []string
	// effectiveAddresses 生效的地址列表
	effectiveAddresses []string
	// 权重数组，和resource的集合一一对应，每一个weight，代表占用桶的份数
	weights []float64
	// 总权重，桶会被划分为totalWeight份
	totalWeight float64
	// 离散因子
	shuffle string
}

func (w *WeightTrafficSelector) BoundOwner() *TrafficSelectorController {
	return w.owner
}

// CalculateAllowedResource 计算被允许的执行资源
func (w *WeightTrafficSelector) CalculateAllowedResource(_ *base.EntryContext) (reource string, effectiveAddresses string) {
	//var ts = strconv.FormatInt(time.Now().UnixNano(), 10)
	//var hashVal = splitBucket(ts, w.shuffle)
	var hashVal = rand.Int63()
	// 将比例分成weight份，看每次请求落在某份上
	var bucket int64
	if w.totalWeight > 0 {
		bucket = hashVal%int64(w.totalWeight) + 1
	} else {
		bucket = 1
	}
	// eg: bucket = 83
	for i := 0; i < len(w.weights); i++ {
		if bucket <= int64(w.weights[i]) {
			return w.resources[i], w.effectiveAddresses[i]
		}
	}
	return "", ""
}

func NewWeightTrafficSelector(owner *TrafficSelectorController, rule *Rule) TrafficSelector {
	if rule == nil {
		logging.Warn("[NewWeightTrafficSelector] rule is nil")
		return nil
	}
	if rule.RouterStrategy != WeightRouter {
		return nil
	}
	if len(rule.GrayWeightList) == 0 {
		// 当权重数组为空是，退化为原始请求资源
		logging.Warn("[NewWeightTrafficSelector] gray weight list len is 0")
		if rule.Force {
			// force=true: 当路由结果为空，直接返回nil
			return nil
		}
		//rule.GrayWeightList = append(rule.GrayWeightList, GWeight{Weight: 1.0, EffectiveAddresses: "[0.0.0.0:*]", TargetResource: rule.Resource, TargetVersion: ""})
	}
	weightTrafficSelector := &WeightTrafficSelector{owner: owner}
	var resources = make([]string, len(rule.GrayWeightList))
	var effectiveAddresses = make([]string, len(rule.GrayWeightList))
	var weights = make([]float64, len(rule.GrayWeightList))

	var totalWeight = 0.0
	for idx, gweight := range rule.GrayWeightList {
		totalWeight += gweight.Weight
		var resource = gweight.TargetResource
		if strings.TrimSpace(gweight.TargetVersion) != "" {
			resource += "." + strings.TrimSpace(gweight.TargetVersion)
		}
		resources[idx] = resource
		effectiveAddresses[idx] = gweight.EffectiveAddresses
		weights[idx] = totalWeight
	}
	weightTrafficSelector.resources = resources
	weightTrafficSelector.effectiveAddresses = effectiveAddresses
	weightTrafficSelector.weights = weights
	weightTrafficSelector.totalWeight = totalWeight
	// 设置离散因子
	weightTrafficSelector.shuffle = uuid.New()
	return weightTrafficSelector
}

// splitBucket 计算val应该划分的桶
func splitBucket(val, shuffle string) int64 {
	var key = val + shuffle
	sum := md5.Sum([]byte(key))
	// hex转字符串
	var hexStr = hex.EncodeToString(sum[:])
	var hash, _ = strconv.ParseInt(hexStr[len(hexStr)-16:len(hexStr)-1], 16, 64)
	if hash < 0 {
		hash = hash * (-1)
	}
	return hash
}
