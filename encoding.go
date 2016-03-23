package bigcache

import (
	"encoding/binary"
)

const (
	timestampSizeInBytes = 8                                                       // Number of bytes used for timestamp
	hashSizeInBytes      = 8                                                       // Number of bytes used for hash
	keySizeInBytes       = 2                                                       // Number of bytes used for size of entry key
	headersSizeInBytes   = timestampSizeInBytes + hashSizeInBytes + keySizeInBytes // Number of bytes used for all headers
)

func wrapEntry(timestamp uint64, hash uint64, key string, entry []byte) []byte {
	var blob []byte
	keyLength := len(key)
	blob = make([]byte, len(entry)+headersSizeInBytes+keyLength)
	binary.LittleEndian.PutUint64(blob, timestamp)
	binary.LittleEndian.PutUint64(blob[timestampSizeInBytes:], hash)
	binary.LittleEndian.PutUint16(blob[timestampSizeInBytes+hashSizeInBytes:], uint16(keyLength))
	copy(blob[headersSizeInBytes:], []byte(key))
	copy(blob[headersSizeInBytes+keyLength:], entry)

	return blob
}

func readEntry(data []byte) []byte {
	length := binary.LittleEndian.Uint16(data[timestampSizeInBytes+hashSizeInBytes:])
	return data[headersSizeInBytes+length:]
}

func readTimestampFromEntry(data []byte) uint64 {
	return binary.LittleEndian.Uint64(data)
}

func readKeyFromEntry(data []byte) string {
	length := binary.LittleEndian.Uint16(data[timestampSizeInBytes+hashSizeInBytes:])
	return string(data[headersSizeInBytes : headersSizeInBytes+length])
}

func readHashFromEntry(data []byte) uint64 {
	return binary.LittleEndian.Uint64(data[timestampSizeInBytes:])
}

func resetKeyFromEntry(data []byte) {
	binary.LittleEndian.PutUint64(data[timestampSizeInBytes:], 0)
}
