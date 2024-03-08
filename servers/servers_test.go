// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package servers

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPIs(t *testing.T) {
	SetDefaultConsistentlyPollingInterval(200 * time.Millisecond)
	SetDefaultEventuallyPollingInterval(200 * time.Millisecond)
	SetDefaultConsistentlyDuration(3 * time.Second)
	SetDefaultEventuallyTimeout(7 * time.Second)
	RegisterFailHandler(Fail)
}
