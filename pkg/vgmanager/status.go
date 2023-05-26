/*
Copyright © 2023 Red Hat, Inc.

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

package vgmanager

import (
	"context"
	"fmt"

	lvmv1alpha1 "github.com/openshift/lvm-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VGReconciler) setVolumeGroupProgressingStatus(ctx context.Context, vgName string) error {
	status := &lvmv1alpha1.VGStatus{
		Name:   vgName,
		Status: lvmv1alpha1.VGStatusProgressing,
	}

	// Set devices for the VGStatus.
	if _, err := r.setDevices(status); err != nil {
		return err
	}

	return r.setVolumeGroupStatus(ctx, status)
}

func (r *VGReconciler) setVolumeGroupReadyStatus(ctx context.Context, vgName string) error {
	status := &lvmv1alpha1.VGStatus{
		Name:   vgName,
		Status: lvmv1alpha1.VGStatusReady,
	}

	// Set devices for the VGStatus.
	if _, err := r.setDevices(status); err != nil {
		return err
	}

	return r.setVolumeGroupStatus(ctx, status)
}

func (r *VGReconciler) setVolumeGroupFailedStatus(ctx context.Context, vgName string, reason string) error {
	status := &lvmv1alpha1.VGStatus{
		Name:   vgName,
		Status: lvmv1alpha1.VGStatusFailed,
		Reason: reason,
	}

	// Set devices for the VGStatus.
	// If there is backing volume group, then set as degraded
	devicesExist, err := r.setDevices(status)
	if err != nil {
		return err
	}
	if devicesExist {
		status.Status = lvmv1alpha1.VGStatusDegraded
	}

	return r.setVolumeGroupStatus(ctx, status)
}

func (r *VGReconciler) setVolumeGroupStatus(ctx context.Context, status *lvmv1alpha1.VGStatus) error {
	// Get LVMVolumeGroupNodeStatus and set the relevant VGStatus
	nodeStatus := &lvmv1alpha1.LVMVolumeGroupNodeStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.NodeName,
			Namespace: r.Namespace,
		},
	}

	result, err := ctrl.CreateOrUpdate(ctx, r.Client, nodeStatus, func() error {
		exists := false
		for i, existingVGStatus := range nodeStatus.Spec.LVMVGStatus {
			if existingVGStatus.Name == status.Name {
				exists = true
				nodeStatus.Spec.LVMVGStatus[i] = *status
			}
		}
		if !exists {
			nodeStatus.Spec.LVMVGStatus = append(nodeStatus.Spec.LVMVGStatus, *status)
		}

		return nil
	})
	if err != nil {
		r.Log.Error(err, "failed to create or update LVMVolumeGroupNodeStatus", "name", nodeStatus.Name)
		return err
	} else if result != controllerutil.OperationResultNone {
		r.Log.Info("LVMVolumeGroupNodeStatus modified", "operation", result, "name", nodeStatus.Name)
	} else {
		r.Log.Info("LVMVolumeGroupNodeStatus unchanged")
	}

	return nil
}

func (r *VGReconciler) removeVolumeGroupStatus(ctx context.Context, vgName string) error {
	// Get LVMVolumeGroupNodeStatus and remove the relevant VGStatus
	nodeStatus := &lvmv1alpha1.LVMVolumeGroupNodeStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.NodeName,
			Namespace: r.Namespace,
		},
	}

	exist := false
	index := 0
	result, err := ctrl.CreateOrUpdate(ctx, r.Client, nodeStatus, func() error {
		for i, existingVGStatus := range nodeStatus.Spec.LVMVGStatus {
			if existingVGStatus.Name == vgName {
				exist = true
				index = i
			}
		}

		if exist {
			nodeStatus.Spec.LVMVGStatus = append(nodeStatus.Spec.LVMVGStatus[:index], nodeStatus.Spec.LVMVGStatus[index+1:]...)
		}

		return nil
	})
	if err != nil {
		r.Log.Error(err, "failed to create or update LVMVolumeGroupNodeStatus", "name", nodeStatus.Name)
		return err
	} else if result != controllerutil.OperationResultNone {
		r.Log.Info("LVMVolumeGroupNodeStatus modified", "operation", result, "name", nodeStatus.Name)
	} else {
		r.Log.Info("LVMVolumeGroupNodeStatus unchanged")
	}

	return nil
}

func (r *VGReconciler) setDevices(status *lvmv1alpha1.VGStatus) (bool, error) {
	vgs, err := ListVolumeGroups(r.executor)
	if err != nil {
		return false, fmt.Errorf("failed to list volume groups. %v", err)
	}

	devicesExist := false
	for _, vg := range vgs {
		if status.Name == vg.Name {
			if len(vg.PVs) > 0 {
				devicesExist = true
				status.Devices = vg.PVs
			}
		}
	}

	return devicesExist, nil
}
