package isuports

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
)

// dispenseID creates UUIDv4 IDs without a database or process-local counter.
// Keep the hyphens: legacy IDs are hexadecimal integers, so the namespaces
// cannot overlap. Tenant IDs use a separate AUTO_INCREMENT and are unchanged.
func dispenseID(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return generateUUID(rand.Reader)
}

func generateUUID(random io.Reader) (string, error) {
	var id [16]byte
	if _, err := io.ReadFull(random, id[:]); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	// RFC 9562, section 5.4: version 4 and the RFC variant (binary 10).
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}
