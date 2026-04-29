package signals

// ResetWarnedForTest is a test-only helper that clears the
// once-per-process deprecation-warning cache so tests can verify
// warning behavior across multiple subtests without leaking state.
func ResetWarnedForTest() { resetWarnedForTest() }
