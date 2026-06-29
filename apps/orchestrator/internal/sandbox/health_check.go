package sandbox

import (
	"context"
	"fmt"
	"net"
	"time"
)

// waitUntilHealthyTCP dials host directly until it accepts a connection, or ctx expires. Used by every provider's StartContainera TCP probe works
// for any challenge app regardless of its route structure, so no challenge codebase ever needs a /healthz route added to it.
func waitUntilHealthyTCP(ctx context.Context, host string) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for container at %s: %w", host, ctx.Err())
		default:
		}

		conn, err := net.DialTimeout("tcp", host, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}

		time.Sleep(healthCheckInterval)
	}
}
