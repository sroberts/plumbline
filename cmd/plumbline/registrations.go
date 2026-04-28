package main

// Blank-imports of signal packages so their init() functions register
// detectors into signals.Default. As new level packages land (l3, l4,
// l5), add them here.
import (
	_ "github.com/sroberts/plumbline/internal/signals/l2"
	_ "github.com/sroberts/plumbline/internal/signals/l3"
	_ "github.com/sroberts/plumbline/internal/signals/l4"
	_ "github.com/sroberts/plumbline/internal/signals/l5"
)
