// Copyright © 2020 Peter Wilson
//
// Copyright © 2020 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// This file has been repurposed for simple AWS SSM environment injection.

package main

import (
	"testing"

	"github.com/banzaicloud/bank-vaults/cmd/vault-secrets-webhook/registry"
	cmp "github.com/google/go-cmp/cmp"
	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	fake "k8s.io/client-go/kubernetes/fake"
)

type MockRegistry struct {
	Image imagev1.ImageConfig
}

func (r *MockRegistry) GetImageConfig(_ kubernetes.Interface, _ string, _ *corev1.Container, _ *corev1.PodSpec) (*imagev1.ImageConfig, error) {
	return &r.Image, nil
}

func Test_mutatingWebhook_mutateContainers(t *testing.T) {
	type fields struct {
		k8sClient kubernetes.Interface
		registry  registry.ImageRegistry
	}
	type args struct {
		containers []corev1.Container
		podSpec    *corev1.PodSpec
		ns         string
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		mutated          bool
		wantErr          bool
		wantedContainers []corev1.Container
	}{
		{name: "Will mutate container with command, no args",
			fields: fields{
				k8sClient: fake.NewSimpleClientset(),
				registry: &MockRegistry{
					Image: imagev1.ImageConfig{},
				},
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:    "MyContainer",
						Image:   "myimage",
						Command: []string{"/bin/bash"},
						Args:    nil,
						Env: []corev1.EnvVar{
							{Name: "myvar", Value: "ssm:secrets"},
						},
					},
				},
			},
			wantedContainers: []corev1.Container{
				{
					Name:         "MyContainer",
					Image:        "myimage",
					Command:      []string{"/mutate/ssm-env"},
					Args:         []string{"/bin/bash"},
					VolumeMounts: []corev1.VolumeMount{{Name: "ssm-env", MountPath: "/mutate/"}},
					Env: []corev1.EnvVar{
						{
							Name:  "myvar",
							Value: "ssm:secrets",
						},
						{
							Name:  "SSM_IGNORE_MISSING_SECRETS",
							Value: "false",
						},
						{
							Name:  "SSM_JSON_LOG",
							Value: "false",
						},
					},
				},
			},
			mutated: true,
			wantErr: false,
		},
		{name: "Will mutate container with args, no command",
			fields: fields{
				k8sClient: fake.NewSimpleClientset(),
				registry: &MockRegistry{
					Image: imagev1.ImageConfig{
						Entrypoint: []string{"myEntryPoint"},
					},
				},
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:    "MyContainer",
						Image:   "myimage",
						Command: []string{},
						Args:    nil,
						Env: []corev1.EnvVar{
							{Name: "myvar", Value: "ssm:secrets"},
						},
					},
				},
			},
			wantedContainers: []corev1.Container{
				{
					Name:         "MyContainer",
					Image:        "myimage",
					Command:      []string{"/mutate/ssm-env"},
					Args:         []string{"myEntryPoint"},
					VolumeMounts: []corev1.VolumeMount{{Name: "ssm-env", MountPath: "/mutate/"}},
					Env: []corev1.EnvVar{
						{
							Name:  "myvar",
							Value: "ssm:secrets",
						},
						{
							Name:  "SSM_IGNORE_MISSING_SECRETS",
							Value: "false",
						},
						{
							Name:  "SSM_JSON_LOG",
							Value: "false",
						},
					},
				},
			},
			mutated: true,
			wantErr: false,
		},
		{name: "Will mutate container with no container-command, no entrypoint",
			fields: fields{
				k8sClient: fake.NewSimpleClientset(),
				registry: &MockRegistry{
					Image: imagev1.ImageConfig{
						Cmd: []string{"myCmd"},
					},
				},
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:    "MyContainer",
						Image:   "myimage",
						Command: []string{},
						Args:    nil,
						Env: []corev1.EnvVar{
							{Name: "myvar", Value: "ssm:secrets"},
						},
					},
				},
			},
			wantedContainers: []corev1.Container{
				{
					Name:         "MyContainer",
					Image:        "myimage",
					Command:      []string{"/mutate/ssm-env"},
					Args:         []string{"myCmd"},
					VolumeMounts: []corev1.VolumeMount{{Name: "ssm-env", MountPath: "/mutate/"}},
					Env: []corev1.EnvVar{
						{
							Name:  "myvar",
							Value: "ssm:secrets",
						},
						{
							Name:  "SSM_IGNORE_MISSING_SECRETS",
							Value: "false",
						},
						{
							Name:  "SSM_JSON_LOG",
							Value: "false",
						},
					},
				},
			},
			mutated: true,
			wantErr: false,
		},
		{name: "Will not mutate container without secrets with correct prefix",
			fields: fields{
				k8sClient: fake.NewSimpleClientset(),
				registry: &MockRegistry{
					Image: imagev1.ImageConfig{},
				},
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:    "MyContainer",
						Image:   "myimage",
						Command: []string{"/bin/bash"},
					},
				},
			},
			wantedContainers: []corev1.Container{
				{
					Name:    "MyContainer",
					Image:   "myimage",
					Command: []string{"/bin/bash"},
				},
			},
			mutated: false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &mutatingWebhook{
				k8sClient: tt.fields.k8sClient,
				registry:  tt.fields.registry,
				logger:    logrus.New(),
			}
			got, err := mw.mutateContainers(tt.args.containers, tt.args.podSpec, tt.args.ns)
			if (err != nil) != tt.wantErr {
				t.Errorf("mutatingWebhook.mutateContainers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.mutated {
				t.Errorf("mutatingWebhook.mutateContainers() = %v, want %v", got, tt.mutated)
			}
			if !cmp.Equal(tt.args.containers, tt.wantedContainers) {
				t.Errorf("mutatingWebhook.mutateContainers() = diff %v", cmp.Diff(tt.args.containers, tt.wantedContainers))
			}
		})
	}
}
