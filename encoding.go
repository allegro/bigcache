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

func wrapEntry(timestamp uint64, hash uint64, key string, entry []byte, buffer *[]byte) []byte {
	keyLength := len(key)
	blobLength := len(entry) + headersSizeInBytes + keyLength

	if blobLength > len(*buffer) {
		*buffer = make([]byte, blobLength)
	}
	blob := *buffer

	binary.LittleEndian.PutUint64(blob, timestamp)
	binary.LittleEndian.PutUint64(blob[timestampSizeInBytes:], hash)
	binary.LittleEndian.PutUint16(blob[timestampSizeInBytes+hashSizeInBytes:], uint16(keyLength))
	copy(blob[headersSizeInBytes:], key)
	copy(blob[headersSizeInBytes+keyLength:], entry)

	return blob[:blobLength]
}

func appendToWrappedEntry(timestamp uint64, wrappedEntry []byte, entry []byte, buffer *[]byte) []byte {
	blobLength := len(wrappedEntry) + len(entry)
	if blobLength > len(*buffer) {
		*buffer = make([]byte, blobLength)
	}

	blob := *buffer

	binary.LittleEndian.PutUint64(blob, timestamp)
	copy(blob[timestampSizeInBytes:], wrappedEntry[timestampSizeInBytes:])
	copy(blob[len(wrappedEntry):], entry)

	return blob[:blobLength]
}

func readEntry(data []byte) []byte {
	if len(data) < headersSizeInBytes {
		return nil
	}
	length := binary.LittleEndian.Uint16(data[timestampSizeInBytes+hashSizeInBytes:])
	offset := int(headersSizeInBytes) + int(length)
	if len(data) < offset {
		return nil
	}

	// copy on read
	dst := make([]byte, len(data)-offset)
	copy(dst, data[offset:])

	return dst
}

func readTimestampFromEntry(data []byte) uint64 {
	if len(data) < timestampSizeInBytes {
		return 0
	}
	return binary.LittleEndian.Uint64(data)
}

func readKeyFromEntry(data []byte) string {
	if len(data) < headersSizeInBytes {
		return ""
	}
	length := binary.LittleEndian.Uint16(data[timestampSizeInBytes+hashSizeInBytes:])
	offset := int(headersSizeInBytes) + int(length)
	if len(data) < offset {
		return ""
	}

	// copy on read
	dst := make([]byte, length)
	copy(dst, data[headersSizeInBytes:offset])

	return bytesToString(dst)
}

func compareKeyFromEntry(data []byte, key string) bool {
	if len(data) < headersSizeInBytes {
		return false
	}
	length := binary.LittleEndian.Uint16(data[timestampSizeInBytes+hashSizeInBytes:])
	offset := int(headersSizeInBytes) + int(length)
	if len(data) < offset {
		return false
	}

	return bytesToString(data[headersSizeInBytes:offset]) == key
}

func readHashFromEntry(data []byte) uint64 {
	if len(data) < timestampSizeInBytes+hashSizeInBytes {
		return 0
	}
	return binary.LittleEndian.Uint64(data[timestampSizeInBytes:])
}

func resetHashFromEntry(data []byte) {
	if len(data) >= timestampSizeInBytes+hashSizeInBytes {
		binary.LittleEndian.PutUint64(data[timestampSizeInBytes:], 0)
	}
}

