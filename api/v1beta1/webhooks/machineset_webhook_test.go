/*
Copyright 2021 The Kubernetes Authors.

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

package webhooks

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	utildefaulting "sigs.k8s.io/cluster-api/util/defaulting"
)

func TestMachineSetDefault(t *testing.T) {
	g := NewWithT(t)
	machineSet := &clusterv1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ms",
		},
		Spec: clusterv1.MachineSetSpec{
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					Version: pointer.String("1.19.10"),
				},
			},
		},
	}
	ms := &MachineSet{machineSet}

	t.Run("for MachineSet", utildefaulting.DefaultValidateTest(ms))
	ms.Default()

	g.Expect(ms.Labels[clusterv1.ClusterNameLabel]).To(Equal(ms.Spec.ClusterName))
	g.Expect(ms.Spec.DeletePolicy).To(Equal(string(clusterv1.RandomMachineSetDeletePolicy)))
	g.Expect(ms.Spec.Selector.MatchLabels).To(HaveKeyWithValue(clusterv1.MachineSetNameLabel, "test-ms"))
	g.Expect(ms.Spec.Template.Labels).To(HaveKeyWithValue(clusterv1.MachineSetNameLabel, "test-ms"))
	g.Expect(*ms.Spec.Template.Spec.Version).To(Equal("v1.19.10"))
}

func TestMachineSetLabelSelectorMatchValidation(t *testing.T) {
	tests := []struct {
		name      string
		selectors map[string]string
		labels    map[string]string
		expectErr bool
	}{
		{
			name:      "should return error on mismatch",
			selectors: map[string]string{"foo": "bar"},
			labels:    map[string]string{"foo": "baz"},
			expectErr: true,
		},
		{
			name:      "should return error on missing labels",
			selectors: map[string]string{"foo": "bar"},
			labels:    map[string]string{"": ""},
			expectErr: true,
		},
		{
			name:      "should return error if all selectors don't match",
			selectors: map[string]string{"foo": "bar", "hello": "world"},
			labels:    map[string]string{"foo": "bar"},
			expectErr: true,
		},
		{
			name:      "should not return error on match",
			selectors: map[string]string{"foo": "bar"},
			labels:    map[string]string{"foo": "bar"},
			expectErr: false,
		},
		{
			name:      "should return error for invalid selector",
			selectors: map[string]string{"-123-foo": "bar"},
			labels:    map[string]string{"-123-foo": "bar"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			machineSet := &clusterv1.MachineSet{
				Spec: clusterv1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: tt.selectors,
					},
					Template: clusterv1.MachineTemplateSpec{
						ObjectMeta: clusterv1.ObjectMeta{
							Labels: tt.labels,
						},
					},
				},
			}
			ms := &MachineSet{machineSet}

			if tt.expectErr {
				warnings, err := ms.ValidateCreate()
				g.Expect(err).To(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				warnings, err = ms.ValidateUpdate(ms)
				g.Expect(err).To(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
			} else {
				warnings, err := ms.ValidateCreate()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				warnings, err = ms.ValidateUpdate(ms)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
			}
		})
	}
}

func TestMachineSetClusterNameImmutable(t *testing.T) {
	tests := []struct {
		name           string
		oldClusterName string
		newClusterName string
		expectErr      bool
	}{
		{
			name:           "when the cluster name has not changed",
			oldClusterName: "foo",
			newClusterName: "foo",
			expectErr:      false,
		},
		{
			name:           "when the cluster name has changed",
			oldClusterName: "foo",
			newClusterName: "bar",
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			newMS := &clusterv1.MachineSet{
				Spec: clusterv1.MachineSetSpec{
					ClusterName: tt.newClusterName,
				},
			}

			oldMS := &clusterv1.MachineSet{
				Spec: clusterv1.MachineSetSpec{
					ClusterName: tt.oldClusterName,
				},
			}

			warnings, err := (&MachineSet{newMS}).ValidateUpdate(&MachineSet{oldMS})
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			g.Expect(warnings).To(BeEmpty())
		})
	}
}

func TestMachineSetVersionValidation(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		expectErr bool
	}{
		{
			name:      "should succeed when given a valid semantic version with prepended 'v'",
			version:   "v1.19.2",
			expectErr: false,
		},
		{
			name:      "should return error when given a valid semantic version without 'v'",
			version:   "1.19.2",
			expectErr: true,
		},
		{
			name:      "should return error when given an invalid semantic version",
			version:   "1",
			expectErr: true,
		},
		{
			name:      "should return error when given an invalid semantic version",
			version:   "v1",
			expectErr: true,
		},
		{
			name:      "should return error when given an invalid semantic version",
			version:   "wrong_version",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			machineSet := &clusterv1.MachineSet{
				Spec: clusterv1.MachineSetSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: pointer.String(tt.version),
						},
					},
				},
			}
			ms := &MachineSet{machineSet}

			if tt.expectErr {
				warnings, err := ms.ValidateCreate()
				g.Expect(err).To(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				warnings, err = ms.ValidateUpdate(ms)
				g.Expect(err).To(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
			} else {
				warnings, err := ms.ValidateCreate()
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				warnings, err = ms.ValidateUpdate(ms)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
			}
		})
	}
}

func TestValidateSkippedMachineSetPreflightChecks(t *testing.T) {
	tests := []struct {
		name      string
		ms        *clusterv1.MachineSet
		expectErr bool
	}{
		{
			name:      "should pass if the machine set skip preflight checks annotation is not set",
			ms:        &clusterv1.MachineSet{},
			expectErr: false,
		},
		{
			name: "should pass if not preflight checks are skipped",
			ms: &clusterv1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.MachineSetSkipPreflightChecksAnnotation: "",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "should pass if only valid preflight checks are skipped (single)",
			ms: &clusterv1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.MachineSetSkipPreflightChecksAnnotation: string(clusterv1.MachineSetPreflightCheckKubeadmVersionSkew),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "should pass if only valid preflight checks are skipped (multiple)",
			ms: &clusterv1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.MachineSetSkipPreflightChecksAnnotation: string(clusterv1.MachineSetPreflightCheckKubeadmVersionSkew) + "," + string(clusterv1.MachineSetPreflightCheckControlPlaneIsStable),
					},
				},
			},
			expectErr: false,
		},
		{
			name: "should fail if invalid preflight checks are skipped",
			ms: &clusterv1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.MachineSetSkipPreflightChecksAnnotation: string(clusterv1.MachineSetPreflightCheckKubeadmVersionSkew) + ",invalid-preflight-check-name",
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := validateSkippedMachineSetPreflightChecks(tt.ms)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
