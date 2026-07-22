/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package kubeserver

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var secretList = corev1.SecretList{
	TypeMeta: metav1.TypeMeta{
		Kind:       "SecretList",
		APIVersion: "v1",
	},
	ListMeta: metav1.ListMeta{
		ResourceVersion: "1231415",
	},
	Items: []corev1.Secret{
		newSecret("secret-1", "default"),
		newSecret("secret-2", "default"),
		newSecret("test", "default"),
		newSecret("secret-1", "dev"),
		newSecret("secret-2", "dev"),
	},
}

func newSecret(name, namespace string) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
