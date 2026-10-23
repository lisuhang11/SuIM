// Package idhash hashes an ordered ID list the same way OpenIM hashutil.IdHash does.
// An empty list hashes to 0. Order is part of the hash.
package idhash

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
)

func Hash(ids []string) uint64 {
	if len(ids) == 0 {
		return 0
	}
	data, _ := json.Marshal(ids)
	sum := md5.Sum(data)
	return binary.BigEndian.Uint64(sum[:])
}
