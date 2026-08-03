package action

import "slices"

// ScopeFromGranted turns the scope strings an instance granted into the one
// capability every availability predicate reads.
//
// This is the only place in labdash that interprets a scope string. An action
// site that read `read_api` itself would be a second answer to "may this token
// write", and the two would eventually disagree.
//
// An unrecognised scope set is treated as writable rather than read-only. We
// cannot prove such a credential is read-only, and refusing every mutation on a
// guess is worse than the 403 rewrite that catches the real case.
//
// It never returns ScopeNone: absence of a credential is the caller's fact, not
// something a scope list can express.
func ScopeFromGranted(granted []string) Scope {
	if slices.Contains(granted, string(ScopeWrite)) {
		return ScopeWrite
	}
	if slices.Contains(granted, string(ScopeRead)) {
		return ScopeRead
	}
	return ScopeWrite
}
