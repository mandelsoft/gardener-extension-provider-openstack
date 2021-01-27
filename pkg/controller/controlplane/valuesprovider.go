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

package controlplane

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gutil "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/version"

	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack/helper"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/openstack"
	"github.com/gardener/gardener-extension-provider-openstack/pkg/utils"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiserver/pkg/authentication/user"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
)

var (
	controlPlaneSecrets = &secrets.Secrets{
		CertificateSecretConfigs: map[string]*secrets.CertificateSecretConfig{
			v1beta1constants.SecretNameCACluster: {
				Name:       v1beta1constants.SecretNameCACluster,
				CommonName: "kubernetes",
				CertType:   secrets.CACert,
			},
		},
		SecretConfigsFunc: func(cas map[string]*secrets.Certificate, clusterName string) []secrets.ConfigInterface {
			return []secrets.ConfigInterface{
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:         openstack.CloudControllerManagerName,
						CommonName:   "system:cloud-controller-manager",
						Organization: []string{user.SystemPrivilegedGroup},
						CertType:     secrets.ClientCert,
						SigningCA:    cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       openstack.CloudControllerManagerName + "-server",
						CommonName: openstack.CloudControllerManagerName,
						DNSNames:   kutil.DNSNamesForService(openstack.CloudControllerManagerName, clusterName),
						CertType:   secrets.ServerCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       openstack.CSIProvisionerName,
						CommonName: openstack.UsernamePrefix + openstack.CSIProvisionerName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       openstack.CSIAttacherName,
						CommonName: openstack.UsernamePrefix + openstack.CSIAttacherName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       openstack.CSISnapshotterName,
						CommonName: openstack.UsernamePrefix + openstack.CSISnapshotterName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       openstack.CSIResizerName,
						CommonName: openstack.UsernamePrefix + openstack.CSIResizerName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
				&secrets.ControlPlaneSecretConfig{
					CertificateSecretConfig: &secrets.CertificateSecretConfig{
						Name:       openstack.CSISnapshotControllerName,
						CommonName: openstack.UsernamePrefix + openstack.CSISnapshotControllerName,
						CertType:   secrets.ClientCert,
						SigningCA:  cas[v1beta1constants.SecretNameCACluster],
					},
					KubeConfigRequest: &secrets.KubeConfigRequest{
						ClusterName:  clusterName,
						APIServerURL: v1beta1constants.DeploymentNameKubeAPIServer,
					},
				},
			}
		},
	}

	configChart = &chart.Chart{
		Name: openstack.CloudProviderConfigName,
		Path: filepath.Join(openstack.InternalChartsPath, openstack.CloudProviderConfigName),
		Objects: []*chart.Object{
			{Type: &corev1.Secret{}, Name: openstack.CloudProviderConfigName},
			{Type: &corev1.Secret{}, Name: openstack.CloudProviderDiskConfigName},
		},
	}

	controlPlaneChart = &chart.Chart{
		Name: "seed-controlplane",
		Path: filepath.Join(openstack.InternalChartsPath, "seed-controlplane"),
		SubCharts: []*chart.Chart{
			{
				Name:   openstack.CloudControllerManagerName,
				Images: []string{openstack.CloudControllerManagerImageName},
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: openstack.CloudControllerManagerName},
					{Type: &appsv1.Deployment{}, Name: openstack.CloudControllerManagerName},
					{Type: &corev1.ConfigMap{}, Name: openstack.CloudControllerManagerName + "-observability-config"},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: openstack.CloudControllerManagerName + "-vpa"},
				},
			},
			{
				Name: openstack.CSIControllerName,
				Images: []string{
					openstack.CSIDriverCinderImageName,
					openstack.CSIProvisionerImageName,
					openstack.CSIAttacherImageName,
					openstack.CSISnapshotterImageName,
					openstack.CSIResizerImageName,
					openstack.CSILivenessProbeImageName,
					openstack.CSISnapshotControllerImageName,
				},
				Objects: []*chart.Object{
					// csi-driver-controller
					{Type: &appsv1.Deployment{}, Name: openstack.CSIControllerName},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: openstack.CSIControllerName + "-vpa"},
					{Type: &corev1.ConfigMap{}, Name: openstack.CSIControllerName + "-observability-config"},
					// csi-snapshot-controller
					{Type: &appsv1.Deployment{}, Name: openstack.CSISnapshotControllerName},
					{Type: &autoscalingv1beta2.VerticalPodAutoscaler{}, Name: openstack.CSISnapshotControllerName + "-vpa"},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name: "shoot-system-components",
		Path: filepath.Join(openstack.InternalChartsPath, "shoot-system-components"),
		SubCharts: []*chart.Chart{
			{
				Name: openstack.CloudControllerManagerName,
				Path: filepath.Join(openstack.InternalChartsPath, openstack.CloudControllerManagerName),
				Objects: []*chart.Object{
					{Type: &rbacv1.ClusterRole{}, Name: "system:controller:cloud-node-controller"},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: "system:controller:cloud-node-controller"},
				},
			},
			{
				Name: openstack.CSINodeName,
				Images: []string{
					openstack.CSIDriverCinderImageName,
					openstack.CSINodeDriverRegistrarImageName,
					openstack.CSILivenessProbeImageName,
				},
				Objects: []*chart.Object{
					// csi-driver
					{Type: &appsv1.DaemonSet{}, Name: openstack.CSINodeName},
					{Type: &storagev1beta1.CSIDriver{}, Name: "cinder.csi.openstack.org"},
					{Type: &corev1.ServiceAccount{}, Name: openstack.CSIDriverName},
					{Type: &corev1.Secret{}, Name: openstack.CloudProviderConfigName},
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIDriverName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIDriverName},
					{Type: &policyv1beta1.PodSecurityPolicy{}, Name: strings.Replace(openstack.UsernamePrefix+openstack.CSIDriverName, ":", ".", -1)},
					{Type: extensionscontroller.GetVerticalPodAutoscalerObject(), Name: openstack.CSINodeName},
					// csi-provisioner
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIProvisionerName},
					// csi-attacher
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIAttacherName},
					// csi-snapshot-controller
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotControllerName},
					// csi-snapshotter
					{Type: &apiextensionsv1beta1.CustomResourceDefinition{}, Name: "volumesnapshotclasses.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1beta1.CustomResourceDefinition{}, Name: "volumesnapshotcontents.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1beta1.CustomResourceDefinition{}, Name: "volumesnapshots.snapshot.storage.k8s.io"},
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSISnapshotterName},
					// csi-resizer
					{Type: &rbacv1.ClusterRole{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
					{Type: &rbacv1.Role{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
					{Type: &rbacv1.RoleBinding{}, Name: openstack.UsernamePrefix + openstack.CSIResizerName},
				},
			},
		},
	}

	storageClassChart = &chart.Chart{
		Name: "shoot-storageclasses",
		Path: filepath.Join(openstack.InternalChartsPath, "shoot-storageclasses"),
	}
)

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider(logger logr.Logger) genericactuator.ValuesProvider {
	return &valuesProvider{
		logger: logger.WithName("openstack-values-provider"),
	}
}

// valuesProvider is a ValuesProvider that provides OpenStack-specific values for the 2 charts applied by the generic actuator.
type valuesProvider struct {
	genericactuator.NoopValuesProvider
	logger logr.Logger
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	cpConfig := &api.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, errors.Wrapf(err, "could not decode providerConfig of controlplane '%s'", kutil.ObjectName(cp))
		}
	}

	infraStatus := &api.InfrastructureStatus{}
	if _, _, err := vp.Decoder().Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, errors.Wrapf(err, "could not decode infrastructureProviderStatus of controlplane '%s'", kutil.ObjectName(cp))
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	// Get credentials
	credentials, err := openstack.GetCredentials(ctx, vp.Client(), cp.Spec.SecretRef)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get service account from secret '%s/%s'", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name)
	}

	return getConfigChartValues(cpConfig, infraStatus, cloudProfileConfig, cp, credentials, cluster)
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &api.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.Decoder().Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, errors.Wrapf(err, "could not decode providerConfig of controlplane '%s'", kutil.ObjectName(cp))
		}
	}

	cpConfigSecret := &corev1.Secret{}
	if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, openstack.CloudProviderConfigName), cpConfigSecret); err != nil {
		return nil, err
	}
	checksums[openstack.CloudProviderConfigName] = gutil.ComputeChecksum(cpConfigSecret.Data)

	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.19")
	if err != nil {
		return nil, err
	}

	if !k8sVersionLessThan119 {
		cpDiskConfigSecret := &corev1.Secret{}
		if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, openstack.CloudProviderCSIDiskConfigName), cpDiskConfigSecret); err != nil {
			return nil, err
		}
		checksums[openstack.CloudProviderCSIDiskConfigName] = gutil.ComputeChecksum(cpDiskConfigSecret.Data)
	}

	// TODO: Remove this code in next version. Delete old config
	if err := vp.deleteCCMMonitoringConfig(ctx, cp.Namespace); err != nil {
		return nil, err
	}

	return getControlPlaneChartValues(cpConfig, cp, cluster, checksums, scaledDown)
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
) (map[string]interface{}, error) {
	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.19")
	if err != nil {
		return nil, err
	}

	var cloudProviderDiskConfig []byte
	if !k8sVersionLessThan119 {
		secret := &corev1.Secret{}
		if err := vp.Client().Get(ctx, kutil.Key(cp.Namespace, openstack.CloudProviderCSIDiskConfigName), secret); err != nil {
			return nil, err
		}

		cloudProviderDiskConfig = secret.Data[openstack.CloudProviderConfigDataKey]
		checksums[openstack.CloudProviderCSIDiskConfigName] = gutil.ComputeChecksum(secret.Data)
	}

	return getControlPlaneShootChartValues(cluster, checksums, k8sVersionLessThan119, cloudProviderDiskConfig)
}

// GetStorageClassesChartValues returns the values for the shoot storageclasses chart applied by the generic actuator.
func (vp *valuesProvider) GetStorageClassesChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.19")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"useLegacyProvisioner": k8sVersionLessThan119,
	}, nil
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(
	cpConfig *api.ControlPlaneConfig,
	infraStatus *api.InfrastructureStatus,
	cloudProfileConfig *api.CloudProfileConfig,
	cp *extensionsv1alpha1.ControlPlane,
	c *openstack.Credentials,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	subnet, err := helper.FindSubnetByPurpose(infraStatus.Networks.Subnets, api.PurposeNodes)
	if err != nil {
		return nil, errors.Wrapf(err, "could not determine subnet from infrastructureProviderStatus of controlplane '%s'", kutil.ObjectName(cp))
	}

	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil - cannot determine keystone URL and other parameters")
	}

	keyStoneURL, err := helper.FindKeyStoneURL(cloudProfileConfig.KeyStoneURLs, cloudProfileConfig.KeyStoneURL, cp.Spec.Region)
	if err != nil {
		return nil, err
	}

	values := map[string]interface{}{
		"kubernetesVersion":          cluster.Shoot.Spec.Kubernetes.Version,
		"domainName":                 c.DomainName,
		"tenantName":                 c.TenantName,
		"username":                   c.Username,
		"password":                   c.Password,
		"region":                     cp.Spec.Region,
		"lbProvider":                 cpConfig.LoadBalancerProvider,
		"floatingNetworkID":          infraStatus.Networks.FloatingPool.ID,
		"subnetID":                   subnet.ID,
		"authUrl":                    keyStoneURL,
		"dhcpDomain":                 cloudProfileConfig.DHCPDomain,
		"requestTimeout":             cloudProfileConfig.RequestTimeout,
		"useOctavia":                 cloudProfileConfig.UseOctavia != nil && *cloudProfileConfig.UseOctavia,
		"rescanBlockStorageOnResize": cloudProfileConfig.RescanBlockStorageOnResize != nil && *cloudProfileConfig.RescanBlockStorageOnResize,
		"nodeVolumeAttachLimit":      cloudProfileConfig.NodeVolumeAttachLimit,
	}
	utils.SetStringValue(values, "floatingSubnetID", infraStatus.Networks.FloatingPool.SubnetID)

	// select matching set of classes from cloud profile
	var profileClasses []api.LoadBalancerClass
	for _, pool := range cloudProfileConfig.Constraints.FloatingPools {
		if pool.Name == infraStatus.Networks.FloatingPool.Name {
			if pool.Region == nil && profileClasses == nil {
				profileClasses = pool.LoadBalancerClasses
			}

			if pool.Region != nil && *pool.Region == cp.Spec.Region {
				profileClasses = pool.LoadBalancerClasses
				break
			}
		}
	}

	// if shoot does not specify classes, uses classes from profile
	var classes []api.LoadBalancerClass
	if cpConfig.LoadBalancerClasses == nil {
		classes = append(profileClasses[:0:0], profileClasses...)
	} else {
		classes = append(cpConfig.LoadBalancerClasses[:0:0], cpConfig.LoadBalancerClasses...)
	}

	defaultClass := lookupLoadBalancerClass(classes, api.DefaultLoadBalancerClass, true)
	if defaultClass != nil {
		setDefaultLoadbalancerSpec(values, defaultClass)
	}

	for _, class := range classes {
		if class.Name == api.PrivateLoadBalancerClass {
			utils.SetStringValue(values, "subnetID", class.SubnetID)
			break
		}
	}

	// enforce existence of a VPN class according to settings purely from the profile
	// the shoot creator MUST NOT be able to redefine the settings for the VPN Loadbalancer
	// first: detect the VPN class from the profile
	vpnClass := lookupLoadBalancerClass(profileClasses, api.VPNLoadBalancerClass, true)

	// second: enforce the VPN class for the shoot
	if vpnClass != nil {
		vpnClass.Name = api.VPNLoadBalancerClass
		found := false
		for i, class := range classes {
			if class.Name == api.VPNLoadBalancerClass {
				classes[i] = *vpnClass
				found = true
				break
			}
		}
		if !found {
			classes = append(classes, *vpnClass)
		}
	} else {
		// only test cases where no LoadBalancer Classes are configured
	}

	var floatingClasses []map[string]interface{}
	for _, class := range classes {
		floatingClass := map[string]interface{}{"name": class.Name}

		if !utils.IsEmptyString(class.FloatingSubnetID) && utils.IsEmptyString(class.FloatingNetworkID) {
			floatingClass["floatingNetworkID"] = infraStatus.Networks.FloatingPool.ID
		} else {
			utils.SetStringValue(floatingClass, "floatingNetworkID", class.FloatingNetworkID)
		}

		utils.SetStringValue(floatingClass, "floatingSubnetTags", class.FloatingSubnetTags)
		utils.SetStringValue(floatingClass, "floatingSubnetName", class.FloatingSubnetName)
		utils.SetStringValue(floatingClass, "floatingSubnetID", class.FloatingSubnetID)
		utils.SetStringValue(floatingClass, "subnetID", class.SubnetID)

		floatingClasses = append(floatingClasses, floatingClass)
	}

	if floatingClasses != nil {
		values["floatingClasses"] = floatingClasses
	}

	return values, nil
}

func lookupLoadBalancerClass(classes []api.LoadBalancerClass, name string, defaulted bool) *api.LoadBalancerClass {
	var found *api.LoadBalancerClass
	var def *api.LoadBalancerClass
	for i, class := range classes {
		tmp := class
		if defaulted && (i == 0 || class.Name == api.DefaultLoadBalancerClass) {
			def = &tmp
		}
		if class.Name == name {
			found = &tmp
			break
		}
	}
	if found == nil {
		return def
	}
	return found
}

// setDefaultLoadbalancerSpec completes the default spec according to a dedicated load balancer class
func setDefaultLoadbalancerSpec(values map[string]interface{}, class *api.LoadBalancerClass) {
	utils.SetStringValue(values, "floatingSubnetTags", class.FloatingSubnetTags)
	utils.SetStringValue(values, "floatingSubnetName", class.FloatingSubnetName)
	utils.SetStringValue(values, "floatingSubnetID", class.FloatingSubnetID)
	utils.SetStringValue(values, "subnetID", class.SubnetID)
}

// getControlPlaneChartValues collects and returns the control plane chart values.
func getControlPlaneChartValues(
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	ccm, err := getCCMChartValues(cpConfig, cp, cluster, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	csi, err := getCSIControllerChartValues(cluster, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		openstack.CloudControllerManagerName: ccm,
		openstack.CSIControllerName:          csi,
	}, nil
}

// getCCMChartValues collects and returns the CCM chart values.
func getCCMChartValues(
	cpConfig *api.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	values := map[string]interface{}{
		"enabled":           true,
		"replicas":          extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"clusterName":       cp.Namespace,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"podNetwork":        extensionscontroller.GetPodNetwork(cluster),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + openstack.CloudControllerManagerName:             checksums[openstack.CloudControllerManagerName],
			"checksum/secret-" + openstack.CloudControllerManagerName + "-server": checksums[openstack.CloudControllerManagerName+"-server"],
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider:         checksums[v1beta1constants.SecretNameCloudProvider],
			"checksum/secret-" + openstack.CloudProviderConfigName:                checksums[openstack.CloudProviderConfigName],
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
	}

	if cpConfig.CloudControllerManager != nil {
		values["featureGates"] = cpConfig.CloudControllerManager.FeatureGates
	}

	return values, nil
}

// getCSIControllerChartValues collects and returns the CSIController chart values.
func getCSIControllerChartValues(
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	k8sVersionLessThan119, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, "<", "1.19")
	if err != nil {
		return nil, err
	}

	if k8sVersionLessThan119 {
		return map[string]interface{}{"enabled": false}, nil
	}

	return map[string]interface{}{
		"enabled":  true,
		"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + openstack.CSIProvisionerName:             checksums[openstack.CSIProvisionerName],
			"checksum/secret-" + openstack.CSIAttacherName:                checksums[openstack.CSIAttacherName],
			"checksum/secret-" + openstack.CSISnapshotterName:             checksums[openstack.CSISnapshotterName],
			"checksum/secret-" + openstack.CSIResizerName:                 checksums[openstack.CSIResizerName],
			"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
		},
		"csiSnapshotController": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-" + openstack.CSISnapshotControllerName: checksums[openstack.CSISnapshotControllerName],
			},
		},
	}, nil
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func getControlPlaneShootChartValues(
	cluster *extensionscontroller.Cluster,
	checksums map[string]string,
	k8sVersionLessThan119 bool,
	cloudProviderDiskConfig []byte,
) (map[string]interface{}, error) {
	return map[string]interface{}{
		openstack.CloudControllerManagerName: map[string]interface{}{"enabled": true},
		openstack.CSINodeName: map[string]interface{}{
			"enabled":    !k8sVersionLessThan119,
			"vpaEnabled": gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-" + openstack.CloudProviderCSIDiskConfigName: checksums[openstack.CloudProviderCSIDiskConfigName],
			},
			"cloudProviderConfig": cloudProviderDiskConfig,
		},
	}, nil
}
