package probe

import (
	"io"
)

// drain discards up to 4 KiB of response body so the keep-alive connection
// can be returned to the pool. Anything larger is left for GC.
func drain(r io.Reader) (int64, error) {
	_, err := io.Copy(io.Discard, io.LimitReader(r, 4096))
	return 0, err
}
