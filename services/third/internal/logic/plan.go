package logic

import (
	"math"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

// Plan chooses the part size for size bytes.
// Files that fit in MinPartSize * MaxNumSize use MinPartSize.
// Larger files use the smallest size that stays within MaxNumSize parts.
func Plan(limits Limits, size int64) (partSize int64, count int, err error) {
	if size <= 0 {
		return 0, 0, errorcode.ErrThirdInvalidArgs.WithMsg("size must be greater than 0")
	}
	if limits.MinPartSize <= 0 || limits.MaxPartSize < limits.MinPartSize || limits.MaxNumSize <= 0 {
		return 0, 0, errorcode.ErrInternal.WithMsg("part limits are invalid")
	}
	if limits.MaxNumSize > math.MaxInt32 {
		return 0, 0, errorcode.ErrInternal.WithMsg("part limits are invalid")
	}
	capBytes, ok := mul(limits.MaxPartSize, limits.MaxNumSize)
	if !ok || size > capBytes {
		return 0, 0, errorcode.ErrThirdTooLarge
	}
	minSpan, ok := mul(limits.MinPartSize, limits.MaxNumSize)
	if !ok {
		return 0, 0, errorcode.ErrInternal.WithMsg("part limits are invalid")
	}
	if size <= minSpan {
		partSize = limits.MinPartSize
	} else {
		partSize = size / limits.MaxNumSize
		if size%limits.MaxNumSize != 0 {
			partSize++
		}
	}
	count = int(size / partSize)
	if size%partSize != 0 {
		count++
	}
	if count <= 0 || int64(count) > limits.MaxNumSize {
		return 0, 0, errorcode.ErrThirdTooLarge
	}
	return partSize, count, nil
}

func partLen(size, partSize int64, count, number int) int64 {
	if number < 1 || number > count {
		return -1
	}
	if number == count {
		return size - partSize*int64(count-1)
	}
	return partSize
}

func signCount(maxParts int32, count int) int {
	n := int(maxParts)
	if n <= 0 {
		n = defaultBatch
	}
	if n > maxSignBatch {
		n = maxSignBatch
	}
	if n > count {
		n = count
	}
	return n
}

func mul(a, b int64) (int64, bool) {
	if a < 0 || b < 0 {
		return 0, false
	}
	if a != 0 && b > math.MaxInt64/a {
		return 0, false
	}
	return a * b, true
}
