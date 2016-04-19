package vorbis

import (
	"math"

	"github.com/jfreymuth/go-vorbis/ogg"
)

func (s *setup) decodePacket(r *ogg.BitReader, prev [][]float32) ([][]float32, [][]float32, error) {
	if r.ReadBool() {
		return nil, nil, ogg.ErrCorruptStream
	}
	modeNumber := r.Read8(ilog(len(s.modes) - 1))
	mode := s.modes[modeNumber]
	// decode window type
	blocktype := mode.blockflag
	longWindow := mode.blockflag == 1
	blocksize := s.blocksize[blocktype]
	windowPrev, windowNext := false, false
	if longWindow {
		windowPrev = r.ReadBool()
		windowNext = r.ReadBool()
	}

	// decode floor data
	mapping := s.mappings[mode.mapping]
	floors := make([]floor, s.channels)
	floorData := make([][]uint32, s.channels)
	noResidue := make([]bool, s.channels)
	for ch := 0; ch < s.channels; ch++ {
		floors[ch] = s.floors[mapping.submaps[mapping.mux[ch]].floor]
		floorData[ch] = floors[ch].Decode(r, s.codebooks, uint32(blocksize/2))
		noResidue[ch] = floorData[ch] == nil
	}

	for i := 0; i < int(mapping.couplingSteps); i++ {
		if !noResidue[mapping.magnitude[i]] || !noResidue[mapping.angle[i]] {
			noResidue[mapping.magnitude[i]] = false
			noResidue[mapping.angle[i]] = false
		}
	}

	// decode residue
	var doNotDecode []bool
	residueVectors := make([][]float32, s.channels)
	for i := range mapping.submaps {
		for j := 0; j < s.channels; j++ {
			if mapping.mux[j] == uint8(i) {
				doNotDecode = append(doNotDecode, noResidue[j])
			}
		}
		residue := s.residues[mapping.submaps[i].residue]
		decodedResidue := residue.Decode(r, doNotDecode, blocksize/2, s.codebooks)
		for j := 0; j < s.channels; j++ {
			if mapping.mux[j] == uint8(i) {
				residueVectors[j] = decodedResidue[0]
				decodedResidue = decodedResidue[1:]
			}
		}
	}

	// inverse coupling
	for i := mapping.couplingSteps; i > 0; i-- {
		magnitudeVector := residueVectors[mapping.magnitude[i-1]]
		angleVector := residueVectors[mapping.angle[i-1]]
		for j := range magnitudeVector {
			m := magnitudeVector[j]
			a := angleVector[j]
			if m > 0 {
				if a > 0 {
					m, a = m, m-a
				} else {
					a, m = m, m+a
				}
			} else {
				if a > 0 {
					m, a = m, m+a
				} else {
					a, m = m, m-a
				}
			}
			magnitudeVector[j] = m
			angleVector[j] = a
		}
	}

	// apply floor data
	for ch := range residueVectors {
		if floorData[ch] != nil {
			floors[ch].Apply(residueVectors[ch], floorData[ch])
		}
	}

	// inverse MDCT
	out := make([][]float32, s.channels)
	for ch := range out {
		out[ch] = make([]float32, blocksize)
		imdct(s.lookup[blocktype], residueVectors[ch], out[ch])
	}

	// apply window and overlap
	center := blocksize / 2
	shortCenter := s.blocksize[0] / 2
	offset := s.blocksize[1]/4 - s.blocksize[0]/4
	final := make([][]float32, s.channels)
	next := make([][]float32, s.channels)
	if longWindow {
		for ch := range out {
			//first half
			if windowPrev {
				for i := uint(0); i < center; i++ {
					out[ch][i] *= s.windows[1][i]
				}
			} else {
				for i := uint(0); i < offset; i++ {
					out[ch][i] = 0
				}
				for i := uint(0); i < shortCenter; i++ {
					out[ch][offset+i] *= s.windows[0][i]
				}
			}
			//second half
			if windowNext {
				for i := center; i < blocksize; i++ {
					out[ch][i] *= s.windows[1][i]
				}
			} else {
				for i := uint(0); i < shortCenter; i++ {
					out[ch][center+offset+i] *= s.windows[0][shortCenter+i]
				}
				for i := center + offset + shortCenter; i < blocksize; i++ {
					out[ch][i] = 0
				}
			}
			//
			start := uint(0)
			center := center
			end := blocksize
			if !windowPrev {
				start += offset
			}
			if !windowNext {
				center += offset
				end -= offset
			}
			final[ch], next[ch] = out[ch][start:center], out[ch][center:end]
			//overlap
			if prev != nil {
				for i := range prev[ch] {
					final[ch][i] += prev[ch][i]
				}
			}
		}
	} else /*short window*/ {
		for ch := range out {
			for i := range out[ch] {
				out[ch][i] *= s.windows[0][i]
			}
			next[ch], final[ch] = out[ch][center:], out[ch][:center]
			//overlap
			if prev != nil {
				for j := range final[ch] {
					final[ch][j] += prev[ch][j]
				}
			}
		}
	}
	return final, next, nil
}

func makeWindow(size uint) []float32 {
	window := make([]float32, size)
	for i := range window {
		window[i] = windowFunc((float32(i) + .5) / float32(size/2) * math.Pi / 2)
	}
	return window
}

func windowFunc(x float32) float32 {
	sinx := math.Sin(float64(x))
	return float32(math.Sin(math.Pi / 2 * sinx * sinx))
}
