package builders

import (
	"fmt"
	"strings"

	v1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
)

func newKeystoreInitContainer(
	serviceName string,
	keystore []v1.KeystoreValue,
	resources corev1.ResourceRequirements,
	image v1.ImageSpec,
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
	securityContext *corev1.SecurityContext,
) ([]corev1.VolumeMount, []corev1.Volume, corev1.Container) {
	keystoreFile := strings.ReplaceAll(serviceName, "-", "_")

	// Add volume and volume mount for keystore
	volumes = append(volumes, corev1.Volume{
		Name: "keystore",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "keystore",
		MountPath: fmt.Sprintf("/usr/share/%s/config/%s.keystore", serviceName, keystoreFile),
		SubPath:   fmt.Sprintf("%s.keystore", keystoreFile),
	})

	initContainerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "keystore",
			MountPath: "/tmp/keystore",
		},
	}

	// Add volumes and volume mounts for keystore secrets
	for _, keystoreValue := range keystore {
		volumes = append(volumes, corev1.Volume{
			Name: "keystore-" + keystoreValue.Secret.Name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: keystoreValue.Secret.Name,
				},
			},
		})

		if len(keystoreValue.KeyMappings) == 0 {
			// If no renames are necessary, mount secret key-value pairs directly
			initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
				Name:      "keystore-" + keystoreValue.Secret.Name,
				MountPath: "/tmp/keystoreSecrets/" + keystoreValue.Secret.Name,
			})
		} else {
			keys := helpers.SortedKeys(keystoreValue.KeyMappings)
			for _, oldKey := range keys {
				initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
					Name:      "keystore-" + keystoreValue.Secret.Name,
					MountPath: "/tmp/keystoreSecrets/" + keystoreValue.Secret.Name + "/" + keystoreValue.KeyMappings[oldKey],
					SubPath:   oldKey,
				})
			}
		}
	}

	keystoreInitContainer := corev1.Container{
		Name:            "keystore",
		Image:           image.GetImage(),
		ImagePullPolicy: image.GetImagePullPolicy(),
		Resources:       resources,
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf(`
				#!/usr/bin/env bash
				set -euo pipefail

				if [ ! -f /usr/share/%[1]s/config/%[2]s].keystore ]; then
				  /usr/share/%[1]s/bin/%[1]s-keystore create
				fi
				for i in /tmp/keystoreSecrets/*/*; do
				  key=$(basename $i)
				  echo "Adding file $i to keystore key $key"
				  cat "$i" | /usr/share/%[1]s/bin/%[1]s-keystore add "$key" -x --force
				done

				# Add the bootstrap password since otherwise the opensearch entrypoint tries to do this on startup
				if [ ! -z ${PASSWORD+x} ]; then
				  echo 'Adding env $PASSWORD to keystore as key bootstrap.password'
				  echo "$PASSWORD" | /usr/share/%[1]s/bin/%[1]s-keystore add -x bootstrap.password
				fi

				cp -a /usr/share/%[1]s/config/%[2]s.keystore /tmp/keystore/
				`, serviceName, keystoreFile),
		},
		VolumeMounts:    initContainerVolumeMounts,
		SecurityContext: securityContext,
	}

	return volumeMounts, volumes, keystoreInitContainer
}
