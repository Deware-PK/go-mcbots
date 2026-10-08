package bot

import (
	"crypto/md5"
)

func offlineUUID(name string) [16]byte {
	h := md5.Sum([]byte("OfflinePlayer:" + name))

	// set version 3 (MD5)
	h[6] = (h[6] & 0x0f) | 0x30
	// set variant bits
	h[8] = (h[8] & 0x3f) | 0x80

	return h
}
