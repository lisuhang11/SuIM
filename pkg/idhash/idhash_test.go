package idhash

import "testing"

func TestHash(t *testing.T) {
	if Hash(nil) != 0 || Hash([]string{}) != 0 {
		t.Fatal("empty list must hash to 0")
	}
	ab := Hash([]string{"a", "b"})
	ba := Hash([]string{"b", "a"})
	if ab == ba {
		t.Fatal("order must change the hash")
	}
	// MD5 of ["a","b"], first 8 bytes big-endian. Same bytes OpenIM hashutil.IdHash returns.
	const want uint64 = 0xe53f04d1bcc1428d
	if ab != want {
		t.Fatalf("hash = %x, want %x", ab, want)
	}
}
