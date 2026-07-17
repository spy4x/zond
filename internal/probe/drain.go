package probe

import "io"

// maxDrainBytes bounds the body we read from a probe response so the
// underlying connection can be returned to the pool. Anything larger
// is left for the runtime to GC when the body closes.
const maxDrainBytes = 4096

// drain discards up to maxDrainBytes of a response body.
func drain(r io.Reader) error {
	_, err := io.Copy(io.Discard, io.LimitReader(r, maxDrainBytes))
	return err
}
