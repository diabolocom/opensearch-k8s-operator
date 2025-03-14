package builders

import (
	v1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
)

func newKeystoreInitContainer(
	keystore []v1.KeystoreValue,
	resources corev1.ResourceRequirements,
	image v1.ImageSpec,
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
	securityContext *corev1.SecurityContext,
) ([]corev1.VolumeMount, []corev1.Volume, corev1.Container) {

	// Add volume and volume mount for keystore
	volumes = append(volumes, corev1.Volume{
		Name: "keystore",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      "keystore",
		MountPath: "/usr/share/opensearch/config/opensearch.keystore",
		SubPath:   "opensearch.keystore",
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

		if keystoreValue.KeyMappings == nil || len(keystoreValue.KeyMappings) == 0 {
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
			`
				#!/usr/bin/env bash
				set -euo pipefail

				if [ ! -f /usr/share/opensearch/config/opensearch.keystore ]; then
				  /usr/share/opensearch/bin/opensearch-keystore create
				fi
				for i in /tmp/keystoreSecrets/*/*; do
				  key=$(basename $i)
				  echo "Adding file $i to keystore key $key"
				  /usr/share/opensearch/bin/opensearch-keystore add-file "$key" "$i" --force
				done

				# Add the bootstrap password since otherwise the opensearch entrypoint tries to do this on startup
				if [ ! -z ${PASSWORD+x} ]; then
				  echo 'Adding env $PASSWORD to keystore as key bootstrap.password'
				  echo "$PASSWORD" | /usr/share/opensearch/bin/opensearch-keystore add -x bootstrap.password
				fi

				cp -a /usr/share/opensearch/config/opensearch.keystore /tmp/keystore/
				`,
		},
		VolumeMounts:    initContainerVolumeMounts,
		SecurityContext: securityContext,
	}

	return volumeMounts, volumes, keystoreInitContainer
}
