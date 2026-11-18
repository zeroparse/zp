// Package pass holds the Go backend's Exec-to-Exec passes (legalization and optimization), which
// register themselves in the pass registry of package pass. The passes arrive with their stage;
// the package exists so that the backend layout is complete.
package pass
