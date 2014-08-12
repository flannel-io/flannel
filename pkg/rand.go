package pkg

import (
	"math/rand"
	"time"
)

var rnd *rand.Rand

func init() {
	seed := time.Now().UnixNano()
	rnd = rand.New(rand.NewSource(seed))
}

func RandInt(lo, hi int) int {
	return lo + int(rnd.Int31n(int32(hi-lo)))
}
