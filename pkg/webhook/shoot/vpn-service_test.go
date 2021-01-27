// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
)

var _ = Describe("Mutator", func() {
	meta := metav1.ObjectMeta{Name: VPNServiceName, Namespace: metav1.NamespaceSystem}
	DescribeTable("#mutateVPMService",
		func(service *corev1.Service) {
			mutator := &mutator{}
			err := mutator.mutateVPNService(context.TODO(), service)

			Expect(err).To(Not(HaveOccurred()))
			Expect(service.Annotations).To(HaveKeyWithValue(AnnotationLoadBalancerClass, openstack.VPNLoadBalancerClass))
		},

		Entry("no data", &corev1.Service{ObjectMeta: meta}),
	)
})
