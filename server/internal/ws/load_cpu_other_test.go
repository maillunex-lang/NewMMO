//go:build !linux

package ws

import "time"

func processCPU() time.Duration { return 0 }
