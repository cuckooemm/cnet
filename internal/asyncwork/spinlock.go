// Copyright 2019 Andy Pan. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package asyncwork

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// CAS 自旋锁
type spinLock uint32

func (sl *spinLock) Lock() {
	for !atomic.CompareAndSwapUint32((*uint32)(sl), 0, 1) {
		runtime.Gosched()
	}
}

func (sl *spinLock) Unlock() {
	atomic.StoreUint32((*uint32)(sl), 0)
}

// SpinLock creates a new spin-lock.
func SpinLock() sync.Locker {
	return new(spinLock)
}
