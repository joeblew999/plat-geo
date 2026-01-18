// Package pmtiles provides PMTiles v3 format support for tile generation.
//
// This is a minimal subset of github.com/protomaps/go-pmtiles/pmtiles,
// containing only the functions needed for PMTiles writing. It excludes
// the MBTiles conversion code which depends on SQLite, making this
// package WASM-compatible for Cloudflare Workers deployment.
//
// Source: https://github.com/protomaps/go-pmtiles (BSD-3-Clause)
// Spec: https://github.com/protomaps/PMTiles/blob/main/spec/v3/spec.md
package pmtiles

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
)

// Compression is the compression algorithm applied to individual tiles.
type Compression uint8

const (
	UnknownCompression Compression = 0
	NoCompression      Compression = 1
	Gzip               Compression = 2
	Brotli             Compression = 3
	Zstd               Compression = 4
)

// TileType is the format of individual tile contents.
type TileType uint8

const (
	UnknownTileType TileType = 0
	Mvt             TileType = 1
	Png             TileType = 2
	Jpeg            TileType = 3
	Webp            TileType = 4
	Avif            TileType = 5
)

// HeaderV3LenBytes is the fixed-size binary header.
const HeaderV3LenBytes = 127

// HeaderV3 is a binary header for PMTiles v3.
type HeaderV3 struct {
	SpecVersion         uint8
	RootOffset          uint64
	RootLength          uint64
	MetadataOffset      uint64
	MetadataLength      uint64
	LeafDirectoryOffset uint64
	LeafDirectoryLength uint64
	TileDataOffset      uint64
	TileDataLength      uint64
	AddressedTilesCount uint64
	TileEntriesCount    uint64
	TileContentsCount   uint64
	Clustered           bool
	InternalCompression Compression
	TileCompression     Compression
	TileType            TileType
	MinZoom             uint8
	MaxZoom             uint8
	MinLonE7            int32
	MinLatE7            int32
	MaxLonE7            int32
	MaxLatE7            int32
	CenterZoom          uint8
	CenterLonE7         int32
	CenterLatE7         int32
}

// EntryV3 is an entry in a PMTiles v3 directory.
type EntryV3 struct {
	TileID    uint64
	Offset    uint64
	Length    uint32
	RunLength uint32
}

// ZxyToID converts (Z,X,Y) tile coordinates to a Hilbert TileID.
func ZxyToID(z uint8, x uint32, y uint32) uint64 {
	var acc uint64 = (1<<(z*2) - 1) / 3
	n := uint32(z - 1)
	for s := uint32(1 << n); s > 0; s >>= 1 {
		var rx = s & x
		var ry = s & y
		acc += uint64((3*rx)^ry) << n
		x, y = rotate(s, x, y, rx, ry)
		n--
	}
	return acc
}

func rotate(n uint32, x uint32, y uint32, rx uint32, ry uint32) (uint32, uint32) {
	if ry == 0 {
		if rx != 0 {
			x = n - 1 - x
			y = n - 1 - y
		}
		return y, x
	}
	return x, y
}

// SerializeHeader converts a header to bytes.
func SerializeHeader(header HeaderV3) []byte {
	b := make([]byte, HeaderV3LenBytes)
	copy(b[0:7], "PMTiles")

	b[7] = 3
	binary.LittleEndian.PutUint64(b[8:8+8], header.RootOffset)
	binary.LittleEndian.PutUint64(b[16:16+8], header.RootLength)
	binary.LittleEndian.PutUint64(b[24:24+8], header.MetadataOffset)
	binary.LittleEndian.PutUint64(b[32:32+8], header.MetadataLength)
	binary.LittleEndian.PutUint64(b[40:40+8], header.LeafDirectoryOffset)
	binary.LittleEndian.PutUint64(b[48:48+8], header.LeafDirectoryLength)
	binary.LittleEndian.PutUint64(b[56:56+8], header.TileDataOffset)
	binary.LittleEndian.PutUint64(b[64:64+8], header.TileDataLength)
	binary.LittleEndian.PutUint64(b[72:72+8], header.AddressedTilesCount)
	binary.LittleEndian.PutUint64(b[80:80+8], header.TileEntriesCount)
	binary.LittleEndian.PutUint64(b[88:88+8], header.TileContentsCount)
	if header.Clustered {
		b[96] = 0x1
	}
	b[97] = uint8(header.InternalCompression)
	b[98] = uint8(header.TileCompression)
	b[99] = uint8(header.TileType)
	b[100] = header.MinZoom
	b[101] = header.MaxZoom
	binary.LittleEndian.PutUint32(b[102:102+4], uint32(header.MinLonE7))
	binary.LittleEndian.PutUint32(b[106:106+4], uint32(header.MinLatE7))
	binary.LittleEndian.PutUint32(b[110:110+4], uint32(header.MaxLonE7))
	binary.LittleEndian.PutUint32(b[114:114+4], uint32(header.MaxLatE7))
	b[118] = header.CenterZoom
	binary.LittleEndian.PutUint32(b[119:119+4], uint32(header.CenterLonE7))
	binary.LittleEndian.PutUint32(b[123:123+4], uint32(header.CenterLatE7))
	return b
}

// SerializeMetadata converts metadata JSON to compressed bytes.
func SerializeMetadata(metadata map[string]interface{}, compression Compression) ([]byte, error) {
	jsonBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	if compression == NoCompression {
		return jsonBytes, nil
	} else if compression == Gzip {
		var b bytes.Buffer
		w, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
		if err != nil {
			return nil, err
		}
		w.Write(jsonBytes)
		w.Close()
		return b.Bytes(), nil
	}
	return nil, errors.New("compression not supported")
}

type nopWriteCloser struct {
	*bytes.Buffer
}

func (w *nopWriteCloser) Close() error { return nil }

// DeserializeHeader parses a binary header.
func DeserializeHeader(d []byte) (HeaderV3, error) {
	h := HeaderV3{}
	if len(d) < HeaderV3LenBytes {
		return h, errors.New("buffer too small for header")
	}
	if string(d[0:7]) != "PMTiles" {
		return h, errors.New("magic number not detected")
	}

	h.SpecVersion = d[7]
	h.RootOffset = binary.LittleEndian.Uint64(d[8 : 8+8])
	h.RootLength = binary.LittleEndian.Uint64(d[16 : 16+8])
	h.MetadataOffset = binary.LittleEndian.Uint64(d[24 : 24+8])
	h.MetadataLength = binary.LittleEndian.Uint64(d[32 : 32+8])
	h.LeafDirectoryOffset = binary.LittleEndian.Uint64(d[40 : 40+8])
	h.LeafDirectoryLength = binary.LittleEndian.Uint64(d[48 : 48+8])
	h.TileDataOffset = binary.LittleEndian.Uint64(d[56 : 56+8])
	h.TileDataLength = binary.LittleEndian.Uint64(d[64 : 64+8])
	h.AddressedTilesCount = binary.LittleEndian.Uint64(d[72 : 72+8])
	h.TileEntriesCount = binary.LittleEndian.Uint64(d[80 : 80+8])
	h.TileContentsCount = binary.LittleEndian.Uint64(d[88 : 88+8])
	h.Clustered = (d[96] == 0x1)
	h.InternalCompression = Compression(d[97])
	h.TileCompression = Compression(d[98])
	h.TileType = TileType(d[99])
	h.MinZoom = d[100]
	h.MaxZoom = d[101]
	h.MinLonE7 = int32(binary.LittleEndian.Uint32(d[102 : 102+4]))
	h.MinLatE7 = int32(binary.LittleEndian.Uint32(d[106 : 106+4]))
	h.MaxLonE7 = int32(binary.LittleEndian.Uint32(d[110 : 110+4]))
	h.MaxLatE7 = int32(binary.LittleEndian.Uint32(d[114 : 114+4]))
	h.CenterZoom = d[118]
	h.CenterLonE7 = int32(binary.LittleEndian.Uint32(d[119 : 119+4]))
	h.CenterLatE7 = int32(binary.LittleEndian.Uint32(d[123 : 123+4]))

	return h, nil
}

// SerializeEntries converts directory entries to compressed bytes.
func SerializeEntries(entries []EntryV3, compression Compression) []byte {
	var b bytes.Buffer
	var w io.WriteCloser

	tmp := make([]byte, binary.MaxVarintLen64)
	if compression == NoCompression {
		w = &nopWriteCloser{&b}
	} else if compression == Gzip {
		w, _ = gzip.NewWriterLevel(&b, gzip.BestCompression)
	} else {
		panic("Compression not supported")
	}

	var n int
	n = binary.PutUvarint(tmp, uint64(len(entries)))
	w.Write(tmp[:n])

	lastID := uint64(0)
	for _, entry := range entries {
		n = binary.PutUvarint(tmp, uint64(entry.TileID)-lastID)
		w.Write(tmp[:n])
		lastID = uint64(entry.TileID)
	}

	for _, entry := range entries {
		n := binary.PutUvarint(tmp, uint64(entry.RunLength))
		w.Write(tmp[:n])
	}

	for _, entry := range entries {
		n := binary.PutUvarint(tmp, uint64(entry.Length))
		w.Write(tmp[:n])
	}

	for i, entry := range entries {
		var n int
		if i > 0 && entry.Offset == entries[i-1].Offset+uint64(entries[i-1].Length) {
			n = binary.PutUvarint(tmp, 0)
		} else {
			n = binary.PutUvarint(tmp, uint64(entry.Offset+1))
		}
		w.Write(tmp[:n])
	}

	w.Close()
	return b.Bytes()
}
