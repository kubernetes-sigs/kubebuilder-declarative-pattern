//go:build !without_exec_applier || !without_direct_applier
// +build !without_exec_applier !without_direct_applier

package applier

var DefaultApplier = NewDirectApplier()
