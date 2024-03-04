// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package unix

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func Listen(ctx context.Context, addr string) (net.Listener, error) {
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(ctx, "unix", addr)
	if errors.Is(err, unix.EADDRINUSE) {
		var si os.FileInfo
		si, err = os.Stat(addr)
		if err != nil {
			return nil, fmt.Errorf("could not stat() socket: %w", err)
		}
		if si.Mode().Type()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("file exists but is not a socket")
		}
		err = os.Remove(addr)
		if err != nil {
			return nil, fmt.Errorf("could not unlink socket: %w", err)
		}
		return lc.Listen(ctx, "unix", addr)
	}
	return ln, nil
}
