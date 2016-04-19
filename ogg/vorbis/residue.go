package vorbis

import (
	"github.com/jfreymuth/go-vorbis/ogg"
)

type residue struct {
	residueType     uint16
	begin, end      uint32
	partitionSize   uint32
	classifications uint8
	classbook       uint8
	cascade         []uint8
	books           [][8]uint8
}

func (x *residue) ReadFrom(r *ogg.BitReader) error {
	x.residueType = r.Read16(16)
	if x.residueType > 2 {
		return ogg.ErrCorruptStream
	}
	x.begin = r.Read32(24)
	x.end = r.Read32(24)
	x.partitionSize = r.Read32(24) + 1
	x.classifications = r.Read8(6) + 1
	x.classbook = r.Read8(8)
	x.cascade = make([]uint8, x.classifications)
	for i := range x.cascade {
		highBits := uint8(0)
		lowBits := r.Read8(3)
		if r.ReadBool() {
			highBits = r.Read8(5)
		}
		x.cascade[i] = highBits*8 + lowBits
	}

	x.books = make([][8]uint8, x.classifications)
	for i := range x.books {
		for j := 0; j < 8; j++ {
			if x.cascade[i]&(1<<uint(j)) != 0 {
				x.books[i][j] = r.Read8(8)
			} else {
				x.books[i][j] = 0xFF //unused
			}
		}
	}

	return nil
}

func (x *residue) Decode(r *ogg.BitReader, doNotDecode []bool, n uint, books []codebook) [][]float32 {
	if x.residueType < 2 {
		return x.decode(r, doNotDecode, n, books)
	} else {
		ch := uint(len(doNotDecode))
		decode := false
		for _, not := range doNotDecode {
			if !not {
				decode = true
				break
			}
		}
		if !decode {
			result := make([][]float32, ch)
			for j := range result {
				result[j] = make([]float32, n)
			}
			return result
		}
		dec := x.decode(r, []bool{false}, n*ch, books)
		result := make([][]float32, ch)
		for i := range result {
			result[i] = make([]float32, n)
		}
		for i := uint(0); i < n; i++ {
			for j := uint(0); j < ch; j++ {
				result[j][i] = dec[0][j+i*ch]
			}
		}
		return result
	}
}

func (x *residue) decode(r *ogg.BitReader, doNotDecode []bool, n uint, books []codebook) [][]float32 {
	ch := uint32(len(doNotDecode))
	actualSize := uint32(n)
	if x.residueType == 2 {
		actualSize *= ch
	}
	begin, end := x.begin, x.end
	if begin > actualSize {
		begin = actualSize
	}
	if end > actualSize {
		end = actualSize
	}
	classbook := books[x.classbook]
	classWordsPerCodeword := classbook.dimensions
	nToRead := end - begin
	partitionsToRead := nToRead / x.partitionSize

	result := make([][]float32, ch)
	for i := range result {
		result[i] = make([]float32, n)
	}
	if nToRead == 0 {
		return result
	}
	cs := (partitionsToRead + classWordsPerCodeword)
	classifications := make([]uint32, ch*cs)
	for pass := 0; pass < 8; pass++ {
		partitionCount := uint32(0)
		for partitionCount < partitionsToRead {
			if pass == 0 {
				for j := uint32(0); j < ch; j++ {
					if !doNotDecode[j] {
						temp := classbook.DecodeScalar(r)
						for i := classWordsPerCodeword; i > 0; i-- {
							classifications[j*cs+(i-1)+partitionCount] = temp % uint32(x.classifications)
							temp /= uint32(x.classifications)
						}
					}
				}
			}
			for i := uint32(0); i < classWordsPerCodeword && partitionCount < partitionsToRead; i++ {
				for j := uint32(0); j < ch; j++ {
					if !doNotDecode[j] {
						vqclass := classifications[j*cs+partitionCount]
						vqbook := x.books[vqclass][pass]
						if vqbook != 0xFF {
							book := books[vqbook]
							offset := begin + partitionCount*x.partitionSize
							if x.residueType == 0 {
								step := x.partitionSize / book.dimensions
								for i := uint32(0); i < step; i++ {
									tmp := book.DecodeVector(r)
									for k := range tmp {
										result[j][offset+i+uint32(k)*step] += tmp[k]
									}
								}
							} else {
								var i uint32
								for i < uint32(x.partitionSize) {
									tmp := book.DecodeVector(r)
									for k := range tmp {
										result[j][offset+i] += tmp[k]
										i++
									}
								}
							}
						}
					}
				}
				partitionCount++
			}
		}
	}
	return result
}
