// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package ssa

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Apply(applyConfig interface{}) client.Patch {
	return applyPatch{
		applyConfig: applyConfig,
	}
}

type applyPatch struct {
	applyConfig interface{}
}

func (p applyPatch) Type() types.PatchType {
	return types.ApplyPatchType
}

func (p applyPatch) Data(_ client.Object) ([]byte, error) {
	return json.Marshal(p.applyConfig)
}
