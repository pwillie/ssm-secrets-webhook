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
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func (mw *mutatingWebhook) mutatePod(pod *corev1.Pod, ns string, dryRun bool) error {
	mw.logger.Debug("Successfully connected to the API")

	initContainersMutated, err := mw.mutateContainers(pod.Spec.InitContainers, &pod.Spec, ns)
	if err != nil {
		return err
	}

	if initContainersMutated {
		mw.logger.Debug("Successfully mutated pod init containers")
	} else {
		mw.logger.Debug("No pod init containers were mutated")
	}

	containersMutated, err := mw.mutateContainers(pod.Spec.Containers, &pod.Spec, ns)
	if err != nil {
		return err
	}

	if containersMutated {
		mw.logger.Debug("Successfully mutated pod containers")
	} else {
		mw.logger.Debug("No pod containers were mutated")
	}

	containerEnvVars := []corev1.EnvVar{}
	containerVolMounts := []corev1.VolumeMount{
		{
			Name:      "ssm-env",
			MountPath: "/mutate/",
		},
	}

	if initContainersMutated || containersMutated {
		pod.Spec.InitContainers = append(getInitContainers(pod.Spec.Containers, pod.Spec.SecurityContext, initContainersMutated, containersMutated, containerEnvVars, containerVolMounts), pod.Spec.InitContainers...)
		mw.logger.Debug("Successfully appended pod init containers to spec")

		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "ssm-env",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		})
		mw.logger.Debug("Successfully appended pod spec volume")
	}

	return nil
}

func hasPodSecurityContextRunAsUser(p *corev1.PodSecurityContext) bool {
	return p.RunAsUser != nil
}

func getServiceAccountMount(containers []corev1.Container) (serviceAccountMount corev1.VolumeMount) {
mountSearch:
	for _, container := range containers {
		for _, mount := range container.VolumeMounts {
			if mount.MountPath == "/var/run/secrets/kubernetes.io/serviceaccount" {
				serviceAccountMount = mount
				break mountSearch
			}
		}
	}
	return serviceAccountMount
}

func getInitContainers(originalContainers []corev1.Container, podSecurityContext *corev1.PodSecurityContext, initContainersMutated bool, containersMutated bool, containerEnvVars []corev1.EnvVar, containerVolMounts []corev1.VolumeMount) []corev1.Container {
	var containers = []corev1.Container{}

	if initContainersMutated || containersMutated {
		containers = append(containers, corev1.Container{
			Name:            "copy-ssm-env",
			Image:           viper.GetString("ssm_env_image"),
			ImagePullPolicy: corev1.PullPolicy(viper.GetString("ssm_env_image_pull_policy")),
			Command:         []string{"sh", "-c", "cp /usr/local/bin/ssm-env /mutate/"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "ssm-env",
					MountPath: "/mutate/",
				},
			},

			SecurityContext: getSecurityContext(podSecurityContext),
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		})
	}

	return containers
}

func getSecurityContext(podSecurityContext *corev1.PodSecurityContext) *corev1.SecurityContext {
	if hasPodSecurityContextRunAsUser(podSecurityContext) {
		return &corev1.SecurityContext{
			RunAsUser: podSecurityContext.RunAsUser,
			// AllowPrivilegeEscalation: &vaultConfig.PspAllowPrivilegeEscalation,
		}
	}

	return &corev1.SecurityContext{
		// AllowPrivilegeEscalation: &vaultConfig.PspAllowPrivilegeEscalation,
	}
}
