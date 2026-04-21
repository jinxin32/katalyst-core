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
	"testing"

	"github.com/kubewharf/katalyst-api/pkg/apis/config/v1alpha1"
	workloadapi "github.com/kubewharf/katalyst-api/pkg/apis/workload/v1alpha1"
	"github.com/kubewharf/katalyst-core/pkg/config/agent/dynamic/adminqos/advisor"
)

func TestCPUProvisionOptions_ApplyTo_RegionIndicatorTimeTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		timeTargetOptions  map[string]string
		wantErr            bool
		wantRegionTypes    []v1alpha1.QoSRegionType
		wantIndicatorNames map[v1alpha1.QoSRegionType][]string
	}{
		{
			name:              "empty options",
			timeTargetOptions: map[string]string{},
			wantErr:           false,
			wantRegionTypes:   nil,
		},
		{
			name: "valid single region single indicator",
			timeTargetOptions: map[string]string{
				"share": `{"cpu_sched_wait":[{"time_range":["09:00","18:00"],"target":400}]}`,
			},
			wantErr:         false,
			wantRegionTypes: []v1alpha1.QoSRegionType{v1alpha1.QoSRegionTypeShare},
			wantIndicatorNames: map[v1alpha1.QoSRegionType][]string{
				v1alpha1.QoSRegionTypeShare: {"cpu_sched_wait"},
			},
		},
		{
			name: "valid single region multiple indicators",
			timeTargetOptions: map[string]string{
				"share": `{"cpu_sched_wait":[{"time_range":["09:00","18:00"],"target":400}],"cpu_usage_ratio":[{"time_range":["09:00","18:00"],"target":0.9}]}`,
			},
			wantErr:         false,
			wantRegionTypes: []v1alpha1.QoSRegionType{v1alpha1.QoSRegionTypeShare},
			wantIndicatorNames: map[v1alpha1.QoSRegionType][]string{
				v1alpha1.QoSRegionTypeShare: {"cpu_sched_wait", "cpu_usage_ratio"},
			},
		},
		{
			name: "valid multiple regions",
			timeTargetOptions: map[string]string{
				"share":     `{"cpu_sched_wait":[{"time_range":["09:00","18:00"],"target":400}]}`,
				"dedicated": `{"cpi":[{"time_range":["09:00","18:00"],"target":1.2}]}`,
			},
			wantErr:         false,
			wantRegionTypes: []v1alpha1.QoSRegionType{v1alpha1.QoSRegionTypeShare, v1alpha1.QoSRegionTypeDedicated},
			wantIndicatorNames: map[v1alpha1.QoSRegionType][]string{
				v1alpha1.QoSRegionTypeShare:     {"cpu_sched_wait"},
				v1alpha1.QoSRegionTypeDedicated: {"cpi"},
			},
		},
		{
			name: "valid multiple time slots per indicator",
			timeTargetOptions: map[string]string{
				"share": `{"cpu_sched_wait":[{"time_range":["09:00","18:00"],"target":400},{"time_range":["18:00","22:00"],"target":300}]}`,
			},
			wantErr:         false,
			wantRegionTypes: []v1alpha1.QoSRegionType{v1alpha1.QoSRegionTypeShare},
			wantIndicatorNames: map[v1alpha1.QoSRegionType][]string{
				v1alpha1.QoSRegionTypeShare: {"cpu_sched_wait"},
			},
		},
		{
			name: "invalid JSON",
			timeTargetOptions: map[string]string{
				"share": `{invalid json}`,
			},
			wantErr: true,
		},
		{
			name: "empty JSON object",
			timeTargetOptions: map[string]string{
				"share": `{}`,
			},
			wantErr:         false,
			wantRegionTypes: []v1alpha1.QoSRegionType{v1alpha1.QoSRegionTypeShare},
			wantIndicatorNames: map[v1alpha1.QoSRegionType][]string{
				v1alpha1.QoSRegionTypeShare: {},
			},
		},
	}

	for _, tt := range tests {
		curTT := tt
		t.Run(curTT.name, func(t *testing.T) {
			t.Parallel()

			options := NewCPUProvisionOptions()
			options.RegionIndicatorTimeTargetOptions = curTT.timeTargetOptions

			config := advisor.NewCPUProvisionConfiguration()
			err := options.ApplyTo(config)

			if (err != nil) != curTT.wantErr {
				t.Errorf("ApplyTo() error = %v, wantErr %v", err, curTT.wantErr)
				return
			}

			if curTT.wantErr {
				return
			}

			if curTT.wantRegionTypes == nil {
				if len(config.RegionIndicatorTimeTargetConfiguration) != 0 {
					t.Errorf("expected no RegionIndicatorTimeTargetConfiguration, got %v", config.RegionIndicatorTimeTargetConfiguration)
				}
				return
			}

			for _, regionType := range curTT.wantRegionTypes {
				strategy, ok := config.RegionIndicatorTimeTargetConfiguration[regionType]
				if !ok {
					t.Errorf("expected region type %v in configuration", regionType)
					continue
				}

				expectedIndicators := curTT.wantIndicatorNames[regionType]
				if len(expectedIndicators) != len(strategy) {
					t.Errorf("region %v: expected %d indicators, got %d", regionType, len(expectedIndicators), len(strategy))
				}
				for _, indicatorName := range expectedIndicators {
					if _, ok := strategy[workloadapi.ServiceSystemIndicatorName(indicatorName)]; !ok {
						t.Errorf("region %v: expected indicator %v not found", regionType, indicatorName)
					}
				}
			}
		})
	}
}

func TestCPUProvisionOptions_ApplyTo_RegionIndicatorTimeTarget_ParsesSlotValues(t *testing.T) {
	t.Parallel()

	options := NewCPUProvisionOptions()
	options.RegionIndicatorTimeTargetOptions = map[string]string{
		"share": `{"cpu_sched_wait":[{"time_range":["09:00","18:00"],"target":400},{"time_range":["18:00","22:00"],"target":300}],"cpu_usage_ratio":[{"time_range":["09:00","18:00"],"target":0.9}]}`,
	}

	config := advisor.NewCPUProvisionConfiguration()
	err := options.ApplyTo(config)
	if err != nil {
		t.Fatalf("ApplyTo() error = %v", err)
	}

	strategy := config.RegionIndicatorTimeTargetConfiguration[v1alpha1.QoSRegionTypeShare]
	if strategy == nil {
		t.Fatal("expected non-nil strategy for share region")
	}

	schedWaitSlots := strategy[workloadapi.ServiceSystemIndicatorNameCPUSchedWait]
	if len(schedWaitSlots) != 2 {
		t.Fatalf("expected 2 slots for cpu_sched_wait, got %d", len(schedWaitSlots))
	}
	if schedWaitSlots[0].Target != 400 {
		t.Errorf("expected target 400 for first slot, got %v", schedWaitSlots[0].Target)
	}
	if !equalHHMMTimeSlices(schedWaitSlots[0].TimeRange, []v1alpha1.HHMMTime{"09:00", "18:00"}) {
		t.Errorf("expected time_range [09:00, 18:00] for first slot, got %v", schedWaitSlots[0].TimeRange)
	}
	if schedWaitSlots[1].Target != 300 {
		t.Errorf("expected target 300 for second slot, got %v", schedWaitSlots[1].Target)
	}
	if !equalHHMMTimeSlices(schedWaitSlots[1].TimeRange, []v1alpha1.HHMMTime{"18:00", "22:00"}) {
		t.Errorf("expected time_range [18:00, 22:00] for second slot, got %v", schedWaitSlots[1].TimeRange)
	}

	usageRatioSlots := strategy[workloadapi.ServiceSystemIndicatorNameCPUUsageRatio]
	if len(usageRatioSlots) != 1 {
		t.Fatalf("expected 1 slot for cpu_usage_ratio, got %d", len(usageRatioSlots))
	}
	if usageRatioSlots[0].Target != 0.9 {
		t.Errorf("expected target 0.9, got %v", usageRatioSlots[0].Target)
	}
}

func TestNewCPUProvisionOptions_HasTimeTargetOptions(t *testing.T) {
	t.Parallel()

	options := NewCPUProvisionOptions()
	if options.RegionIndicatorTimeTargetOptions == nil {
		t.Errorf("RegionIndicatorTimeTargetOptions should not be nil")
	}
	if len(options.RegionIndicatorTimeTargetOptions) != 0 {
		t.Errorf("RegionIndicatorTimeTargetOptions should be empty, got %v", options.RegionIndicatorTimeTargetOptions)
	}
}

func equalHHMMTimeSlices(a, b []v1alpha1.HHMMTime) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
