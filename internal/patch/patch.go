// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package patch

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ApplyConfiguration(applyConf interface{}) client.Patch {
	return applyConfPatch{
		applyConf: applyConf,
	}
}

type applyConfPatch struct {
	applyConf interface{}
}

func (p applyConfPatch) Type() types.PatchType {
	return types.ApplyPatchType
}

func (p applyConfPatch) Data(_ client.Object) ([]byte, error) {
	return json.Marshal(p.applyConf)
}
