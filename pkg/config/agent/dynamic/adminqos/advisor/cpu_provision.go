/*
Copyright 2022 The Katalyst Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package advisor

import (
	"time"

	"github.com/kubewharf/katalyst-api/pkg/apis/config/v1alpha1"
	workloadv1alpha1 "github.com/kubewharf/katalyst-api/pkg/apis/workload/v1alpha1"
	"github.com/kubewharf/katalyst-api/pkg/utils"
	"github.com/kubewharf/katalyst-core/pkg/config/agent/dynamic/crd"
	"github.com/kubewharf/katalyst-core/pkg/consts"
	"github.com/kubewharf/katalyst-core/pkg/util/general"
)

type CPUProvisionConfiguration struct {
	AllowSharedCoresOverlapReclaimedCores       bool
	RegionIndicatorTargetConfiguration          map[v1alpha1.QoSRegionType][]v1alpha1.IndicatorTargetConfiguration
	RegionIndicatorTimeTargetConfiguration      map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots
	IndicatorTargetGetters                      map[string]string
	IndicatorTargetDefaultGetter                string
	IndicatorTargetMetricThresholdExpandFactors map[string]float64
}

func NewCPUProvisionConfiguration() *CPUProvisionConfiguration {
	return &CPUProvisionConfiguration{
		AllowSharedCoresOverlapReclaimedCores: false,
		RegionIndicatorTargetConfiguration: map[v1alpha1.QoSRegionType][]v1alpha1.IndicatorTargetConfiguration{
			v1alpha1.QoSRegionTypeShare: {
				{
					Name:   workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
					Target: 460,
				},
				{
					Name:   workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio,
					Target: 0.8,
				},
			},
			v1alpha1.QoSRegionTypeDedicated: {
				{
					Name:   workloadv1alpha1.ServiceSystemIndicatorNameCPI,
					Target: 1.4,
				},
				{
					Name:   workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio,
					Target: 0.55,
				},
			},
		},
		RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{},
		IndicatorTargetGetters: map[string]string{
			string(workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio): string(consts.IndicatorTargetGetterSPDAvg),
		},
		IndicatorTargetDefaultGetter: string(consts.IndicatorTargetGetterSPDMin),
		IndicatorTargetMetricThresholdExpandFactors: map[string]float64{
			string(workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio): 1,
		},
	}
}

func (c *CPUProvisionConfiguration) ApplyConfiguration(conf *crd.DynamicConfigCRD) {
	if aqc := conf.AdminQoSConfiguration; aqc != nil &&
		aqc.Spec.Config.AdvisorConfig != nil &&
		aqc.Spec.Config.AdvisorConfig.CPUAdvisorConfig != nil {
		if aqc.Spec.Config.AdvisorConfig.CPUAdvisorConfig.CPUProvisionConfig != nil {
			for _, regionIndicator := range aqc.Spec.Config.AdvisorConfig.CPUAdvisorConfig.CPUProvisionConfig.RegionIndicators {
				c.RegionIndicatorTargetConfiguration[utils.CompatibleLegacyRegionType(regionIndicator.RegionType)] = regionIndicator.Targets
			}
			for _, regionTimeTarget := range aqc.Spec.Config.AdvisorConfig.CPUAdvisorConfig.CPUProvisionConfig.RegionIndicatorTimeTargets {
				c.RegionIndicatorTimeTargetConfiguration[utils.CompatibleLegacyRegionType(regionTimeTarget.RegionType)] = regionTimeTarget.IndicatorTimeTargets
			}
		}
		if aqc.Spec.Config.AdvisorConfig.CPUAdvisorConfig.AllowSharedCoresOverlapReclaimedCores != nil {
			c.AllowSharedCoresOverlapReclaimedCores = *aqc.Spec.Config.AdvisorConfig.CPUAdvisorConfig.AllowSharedCoresOverlapReclaimedCores
		}
	}
}

func (c *CPUProvisionConfiguration) GetIndicatorTimeTarget(regionType v1alpha1.QoSRegionType, indicatorName workloadv1alpha1.ServiceSystemIndicatorName) (float64, bool) {
	strategy, ok := c.RegionIndicatorTimeTargetConfiguration[regionType]
	if !ok {
		return 0, false
	}
	slots, ok := strategy[indicatorName]
	if !ok {
		return 0, false
	}
	for _, slot := range slots {
		if target, match := matchTimeSlot(slot); match {
			return target, true
		}
	}
	return 0, false
}

func matchTimeSlot(slot v1alpha1.IndicatorTimeTargetSlot) (float64, bool) {
	if len(slot.TimeRange) != 2 {
		return 0, false
	}
	layout := "15:04"
	now := time.Now()
	loc := now.Location()
	start, err := time.ParseInLocation(layout, string(slot.TimeRange[0]), loc)
	if err != nil {
		return 0, false
	}
	end, err := time.ParseInLocation(layout, string(slot.TimeRange[1]), loc)
	if err != nil {
		return 0, false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	startTime := today.Add(time.Duration(start.Hour())*time.Hour + time.Duration(start.Minute())*time.Minute)
	endTime := today.Add(time.Duration(end.Hour())*time.Hour + time.Duration(end.Minute())*time.Minute)
	if endTime.Before(startTime) {
		general.Warningf("cross-midnight time range is not supported: [%v, %v], the time target will not take effect", slot.TimeRange[0], slot.TimeRange[1])
		return 0, false
	}
	if now.After(startTime) && now.Before(endTime) {
		return slot.Target, true
	}
	return 0, false
}
