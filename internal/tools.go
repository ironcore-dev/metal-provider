// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

//go:build tools
// +build tools

package internal

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/addlicense"
	_ "github.com/ironcore-dev/ironcore/irictl-machine/cmd/irictl-machine"
)
