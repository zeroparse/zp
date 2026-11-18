//go:build !zp_reference_stream_frames

package state

// Build-time selection keeps the reference control out of matching loops.
const StreamFrameScratch = true
