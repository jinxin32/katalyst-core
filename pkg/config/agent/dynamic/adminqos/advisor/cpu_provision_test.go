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
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/kubewharf/katalyst-api/pkg/apis/config/v1alpha1"
	workloadv1alpha1 "github.com/kubewharf/katalyst-api/pkg/apis/workload/v1alpha1"
	"github.com/kubewharf/katalyst-core/pkg/config/agent/dynamic/crd"
)

func TestMatchTimeSlot(t *testing.T) {
	t.Parallel()

	now := time.Now()
	inRangeStart := now.Add(-10 * time.Minute)
	inRangeEnd := now.Add(10 * time.Minute)
	inRangeStartStr := v1alpha1.HHMMTime(inRangeStart.Format("15:04"))
	inRangeEndStr := v1alpha1.HHMMTime(inRangeEnd.Format("15:04"))

	outOfRangeStart := now.Add(1 * time.Hour)
	outOfRangeEnd := now.Add(2 * time.Hour)
	outOfRangeStartStr := v1alpha1.HHMMTime(outOfRangeStart.Format("15:04"))
	outOfRangeEndStr := v1alpha1.HHMMTime(outOfRangeEnd.Format("15:04"))

	tests := []struct {
		name       string
		slot       v1alpha1.IndicatorTimeTargetSlot
		wantTarget float64
		wantMatch  bool
	}{
		{
			name: "in range",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{inRangeStartStr, inRangeEndStr},
				Target:    400,
			},
			wantTarget: 400,
			wantMatch:  true,
		},
		{
			name: "out of range",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{outOfRangeStartStr, outOfRangeEndStr},
				Target:    300,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
		{
			name: "invalid time range length - single element",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{"14:00"},
				Target:    250,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
		{
			name: "invalid time range length - three elements",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{"09:00", "18:00", "22:00"},
				Target:    250,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
		{
			name: "invalid time format - start",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{"25:00", "18:00"},
				Target:    250,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
		{
			name: "invalid time format - end",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{"09:00", "abc"},
				Target:    250,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
		{
			name: "empty time range",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{},
				Target:    250,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
		{
			name: "nil time range",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: nil,
				Target:    250,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
		{
			name: "cross-midnight time range",
			slot: v1alpha1.IndicatorTimeTargetSlot{
				TimeRange: []v1alpha1.HHMMTime{"22:00", "06:00"},
				Target:    300,
			},
			wantTarget: 0,
			wantMatch:  false,
		},
	}

	for _, tt := range tests {
		curTT := tt
		t.Run(curTT.name, func(t *testing.T) {
			t.Parallel()
			gotTarget, gotMatch := matchTimeSlot(curTT.slot)
			if gotTarget != curTT.wantTarget {
				t.Errorf("matchTimeSlot() gotTarget = %v, want %v", gotTarget, curTT.wantTarget)
			}
			if gotMatch != curTT.wantMatch {
				t.Errorf("matchTimeSlot() gotMatch = %v, want %v", gotMatch, curTT.wantMatch)
			}
		})
	}
}

func TestGetIndicatorTimeTarget(t *testing.T) {
	t.Parallel()

	now := time.Now()
	inRangeStart := now.Add(-10 * time.Minute)
	inRangeEnd := now.Add(10 * time.Minute)
	inRangeStartStr := v1alpha1.HHMMTime(inRangeStart.Format("15:04"))
	inRangeEndStr := v1alpha1.HHMMTime(inRangeEnd.Format("15:04"))

	outOfRangeStart := now.Add(1 * time.Hour)
	outOfRangeEnd := now.Add(2 * time.Hour)
	outOfRangeStartStr := v1alpha1.HHMMTime(outOfRangeStart.Format("15:04"))
	outOfRangeEndStr := v1alpha1.HHMMTime(outOfRangeEnd.Format("15:04"))

	tests := []struct {
		name          string
		config        *CPUProvisionConfiguration
		regionType    v1alpha1.QoSRegionType
		indicatorName workloadv1alpha1.ServiceSystemIndicatorName
		wantTarget    float64
		wantMatched   bool
	}{
		{
			name: "matched time target",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
					v1alpha1.QoSRegionTypeShare: {
						workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
							{TimeRange: []v1alpha1.HHMMTime{inRangeStartStr, inRangeEndStr}, Target: 400},
						},
					},
				},
			},
			regionType:    v1alpha1.QoSRegionTypeShare,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
			wantTarget:    400,
			wantMatched:   true,
		},
		{
			name: "no matching time range - out of range",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
					v1alpha1.QoSRegionTypeShare: {
						workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
							{TimeRange: []v1alpha1.HHMMTime{outOfRangeStartStr, outOfRangeEndStr}, Target: 300},
						},
					},
				},
			},
			regionType:    v1alpha1.QoSRegionTypeShare,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
			wantTarget:    0,
			wantMatched:   false,
		},
		{
			name: "region type not found",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
					v1alpha1.QoSRegionTypeShare: {
						workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
							{TimeRange: []v1alpha1.HHMMTime{inRangeStartStr, inRangeEndStr}, Target: 400},
						},
					},
				},
			},
			regionType:    v1alpha1.QoSRegionTypeDedicated,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
			wantTarget:    0,
			wantMatched:   false,
		},
		{
			name: "indicator name not found",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
					v1alpha1.QoSRegionTypeShare: {
						workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
							{TimeRange: []v1alpha1.HHMMTime{inRangeStartStr, inRangeEndStr}, Target: 400},
						},
					},
				},
			},
			regionType:    v1alpha1.QoSRegionTypeShare,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio,
			wantTarget:    0,
			wantMatched:   false,
		},
		{
			name: "empty configuration",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{},
			},
			regionType:    v1alpha1.QoSRegionTypeShare,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
			wantTarget:    0,
			wantMatched:   false,
		},
		{
			name: "multiple indicators - matched one",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
					v1alpha1.QoSRegionTypeShare: {
						workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
							{TimeRange: []v1alpha1.HHMMTime{inRangeStartStr, inRangeEndStr}, Target: 400},
						},
						workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio: {
							{TimeRange: []v1alpha1.HHMMTime{outOfRangeStartStr, outOfRangeEndStr}, Target: 0.9},
						},
					},
				},
			},
			regionType:    v1alpha1.QoSRegionTypeShare,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
			wantTarget:    400,
			wantMatched:   true,
		},
		{
			name: "multiple time slots - first matched",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
					v1alpha1.QoSRegionTypeShare: {
						workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
							{TimeRange: []v1alpha1.HHMMTime{inRangeStartStr, inRangeEndStr}, Target: 400},
							{TimeRange: []v1alpha1.HHMMTime{outOfRangeStartStr, outOfRangeEndStr}, Target: 300},
						},
					},
				},
			},
			regionType:    v1alpha1.QoSRegionTypeShare,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
			wantTarget:    400,
			wantMatched:   true,
		},
		{
			name: "multiple time slots - second matched",
			config: &CPUProvisionConfiguration{
				RegionIndicatorTimeTargetConfiguration: map[v1alpha1.QoSRegionType]map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
					v1alpha1.QoSRegionTypeShare: {
						workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
							{TimeRange: []v1alpha1.HHMMTime{outOfRangeStartStr, outOfRangeEndStr}, Target: 300},
							{TimeRange: []v1alpha1.HHMMTime{inRangeStartStr, inRangeEndStr}, Target: 400},
						},
					},
				},
			},
			regionType:    v1alpha1.QoSRegionTypeShare,
			indicatorName: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait,
			wantTarget:    400,
			wantMatched:   true,
		},
	}

	for _, tt := range tests {
		curTT := tt
		t.Run(curTT.name, func(t *testing.T) {
			t.Parallel()
			gotTarget, gotMatched := curTT.config.GetIndicatorTimeTarget(curTT.regionType, curTT.indicatorName)
			if gotTarget != curTT.wantTarget {
				t.Errorf("GetIndicatorTimeTarget() gotTarget = %v, want %v", gotTarget, curTT.wantTarget)
			}
			if gotMatched != curTT.wantMatched {
				t.Errorf("GetIndicatorTimeTarget() gotMatched = %v, want %v", gotMatched, curTT.wantMatched)
			}
		})
	}
}

func TestNewCPUProvisionConfiguration_HasTimeTargetConfig(t *testing.T) {
	t.Parallel()

	config := NewCPUProvisionConfiguration()
	if config.RegionIndicatorTimeTargetConfiguration == nil {
		t.Errorf("RegionIndicatorTimeTargetConfiguration should not be nil")
	}
	if len(config.RegionIndicatorTimeTargetConfiguration) != 0 {
		t.Errorf("RegionIndicatorTimeTargetConfiguration should be empty, got %v", config.RegionIndicatorTimeTargetConfiguration)
	}
}

func TestIndicatorTimeTargetSlots_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
		workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
			{TimeRange: []v1alpha1.HHMMTime{"09:00", "18:00"}, Target: 400},
			{TimeRange: []v1alpha1.HHMMTime{"18:00", "22:00"}, Target: 300},
		},
		workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio: {
			{TimeRange: []v1alpha1.HHMMTime{"09:00", "18:00"}, Target: 0.9},
		},
	}

	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("JSON round trip failed.\nOriginal: %v\nDecoded: %v", original, decoded)
	}
}

func TestApplyConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("region indicators", func(t *testing.T) {
		t.Parallel()

		config := NewCPUProvisionConfiguration()
		conf := &crd.DynamicConfigCRD{
			AdminQoSConfiguration: &v1alpha1.AdminQoSConfiguration{
				Spec: v1alpha1.AdminQoSConfigurationSpec{
					Config: v1alpha1.AdminQoSConfig{
						AdvisorConfig: &v1alpha1.AdvisorConfig{
							CPUAdvisorConfig: &v1alpha1.CPUAdvisorConfig{
								CPUProvisionConfig: &v1alpha1.CPUProvisionConfig{
									RegionIndicators: []v1alpha1.RegionIndicators{
										{
											RegionType: v1alpha1.QoSRegionTypeShare,
											Targets: []v1alpha1.IndicatorTargetConfiguration{
												{Name: workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait, Target: 500},
												{Name: workloadv1alpha1.ServiceSystemIndicatorNameCPUUsageRatio, Target: 0.85},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		config.ApplyConfiguration(conf)

		targets, ok := config.RegionIndicatorTargetConfiguration[v1alpha1.QoSRegionTypeShare]
		if !ok {
			t.Fatal("RegionIndicatorTargetConfiguration should have RegionTypeShare")
		}
		if len(targets) != 2 {
			t.Fatalf("expected 2 targets, got %d", len(targets))
		}
		if targets[0].Target != 500 {
			t.Errorf("first target should be 500, got %v", targets[0].Target)
		}
		if targets[1].Target != 0.85 {
			t.Errorf("second target should be 0.85, got %v", targets[1].Target)
		}
	})

	t.Run("region indicator time targets", func(t *testing.T) {
		t.Parallel()

		config := NewCPUProvisionConfiguration()
		conf := &crd.DynamicConfigCRD{
			AdminQoSConfiguration: &v1alpha1.AdminQoSConfiguration{
				Spec: v1alpha1.AdminQoSConfigurationSpec{
					Config: v1alpha1.AdminQoSConfig{
						AdvisorConfig: &v1alpha1.AdvisorConfig{
							CPUAdvisorConfig: &v1alpha1.CPUAdvisorConfig{
								CPUProvisionConfig: &v1alpha1.CPUProvisionConfig{
									RegionIndicatorTimeTargets: []v1alpha1.RegionIndicatorTimeTargets{
										{
											RegionType: v1alpha1.QoSRegionTypeShare,
											IndicatorTimeTargets: map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
												workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
													{TimeRange: []v1alpha1.HHMMTime{"09:00", "18:00"}, Target: 400},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		config.ApplyConfiguration(conf)

		regionConfig, ok := config.RegionIndicatorTimeTargetConfiguration[v1alpha1.QoSRegionTypeShare]
		if !ok {
			t.Fatal("RegionIndicatorTimeTargetConfiguration should have RegionTypeShare")
		}
		slots, ok := regionConfig[workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait]
		if !ok {
			t.Fatal("should have CPUSchedWait indicator")
		}
		if len(slots) != 1 {
			t.Fatalf("expected 1 slot, got %d", len(slots))
		}
		if slots[0].Target != 400 {
			t.Errorf("expected target 400, got %v", slots[0].Target)
		}
		if string(slots[0].TimeRange[0]) != "09:00" || string(slots[0].TimeRange[1]) != "18:00" {
			t.Errorf("unexpected time range: %v", slots[0].TimeRange)
		}
	})

	t.Run("nil crd configuration", func(t *testing.T) {
		t.Parallel()

		config := NewCPUProvisionConfiguration()
		config.ApplyConfiguration(&crd.DynamicConfigCRD{})

		if len(config.RegionIndicatorTimeTargetConfiguration) != 0 {
			t.Error("RegionIndicatorTimeTargetConfiguration should be empty")
		}
	})

	t.Run("legacy region type conversion", func(t *testing.T) {
		t.Parallel()

		config := NewCPUProvisionConfiguration()
		conf := &crd.DynamicConfigCRD{
			AdminQoSConfiguration: &v1alpha1.AdminQoSConfiguration{
				Spec: v1alpha1.AdminQoSConfigurationSpec{
					Config: v1alpha1.AdminQoSConfig{
						AdvisorConfig: &v1alpha1.AdvisorConfig{
							CPUAdvisorConfig: &v1alpha1.CPUAdvisorConfig{
								CPUProvisionConfig: &v1alpha1.CPUProvisionConfig{
									RegionIndicatorTimeTargets: []v1alpha1.RegionIndicatorTimeTargets{
										{
											RegionType: "share",
											IndicatorTimeTargets: map[workloadv1alpha1.ServiceSystemIndicatorName]v1alpha1.IndicatorTimeTargetSlots{
												workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait: {
													{TimeRange: []v1alpha1.HHMMTime{"10:00", "14:00"}, Target: 350},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		config.ApplyConfiguration(conf)

		regionConfig, ok := config.RegionIndicatorTimeTargetConfiguration["share"]
		if !ok {
			t.Fatal("RegionIndicatorTimeTargetConfiguration should have 'share' via CompatibleLegacyRegionType")
		}
		slots, ok := regionConfig[workloadv1alpha1.ServiceSystemIndicatorNameCPUSchedWait]
		if !ok {
			t.Fatal("should have CPUSchedWait indicator")
		}
		if slots[0].Target != 350 {
			t.Errorf("expected target 350, got %v", slots[0].Target)
		}
	})
}
