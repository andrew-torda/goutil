// 6 Apr 2020
// seqcalc does simple, common calculations on a set of sequences.
// The functions have to live in this package, since they
// need access to the internals of a sequence

package seq

import (
	"fmt"
	"math"

	"github.com/andrew-torda/matrix"
)

// SetSymUsed fills out the bool slice which says whether or not a
// symbol was used
func (seqgrp *SeqGrp) SetSymUsed() {
	for _, ss := range seqgrp.seqs {
		s := ss.GetSeq()
		for _, c := range s {
			seqgrp.symUsed[c] = true
		}
	}
	seqgrp.usedKnwn = true
}

// GetType looks at a set of sequences and returns its best guess
// as to the type of file.
func (seqgrp *SeqGrp) GetType() SeqType {
	if seqgrp.stype != Unknown { // If the sequence type has been
		return seqgrp.stype //      set, just return it.
	}

	if seqgrp.usedKnwn != true {
		seqgrp.SetSymUsed()
	}
	protType := []byte{
		'D', 'E', 'F', 'H', 'I', 'K', 'L', 'M',
		'N', 'P', 'Q', 'R', 'S', 'V', 'W', 'Y'}

	used := seqgrp.symUsed
	for _, c := range protType { // If we see an amino acid code,
		if used[c] { //          just return protein type.
			return Protein
		}
	}

	if used['T'] && used['U'] {
		return Ntide
	}
	// If we have ACG, but neither T or U, it is a nucleotide
	// but we cannot tell if it is RNA or DNA
	if used['A'] && used['C'] && used['G'] && !used['T'] && !used['U'] {
		return Ntide
	}
	if used['T'] {
		return DNA
	}
	if used['U'] {
		return RNA
	}

	return Unknown
}

// mapsyms looks at the symbols(characters, bases, residues) used in a
// seqgrp. It then makes a little array for mapping.
func (seqgrp *SeqGrp) mapsyms() error {
	if seqgrp.usedKnwn != true {
		seqgrp.SetSymUsed()
	}	
	for i := range seqgrp.mapping { // Initialise with bad value, to
		seqgrp.mapping [i] = math.MaxUint8 // trap errors later
	}

	var n uint8
	for i := range seqgrp.symUsed {
		if seqgrp.symUsed[i] {
			seqgrp.mapping[i] = n
			if n >= math.MaxUint8 {
				panic("program bug")
			}
			seqgrp.revmap = append(seqgrp.revmap, uint8(i))
			n++
		}
	}
	return nil
}

// Usage by site counts how many of each symbol/character appear at
// each site in the alignment
// counts.Mat looks like [length_of_seq][number_of_types]
func (seqgrp *SeqGrp) UsageSite() {
	if len(seqgrp.revmap) == 0 {
		seqgrp.mapsyms()
	}
	nrow := len(seqgrp.revmap)
	ncol := len(seqgrp.seqs[0].GetSeq())
	seqgrp.counts = matrix.NewFMatrix2d(nrow, ncol)
	for _, ss := range seqgrp.seqs { // Breaking here
		for i, c := range ss.GetSeq() {
			cmap := seqgrp.mapping[c]
			seqgrp.counts.Mat[cmap][i] += 1
		}
	}
}

// Usage Frac converts count to normalised frequencies. If letter 'A'
// occurs 2 times in five positions, its count entry will be change from
// 2 to 2/5 = 0.4
// If gapsAreChar is true, gaps ("-") are treated as a valid character
// type. Otherwise they are removed from the tallies.
func (seqgrp *SeqGrp) UsageFrac(gapsAreChar bool) {
	if seqgrp.counts == nil {
		seqgrp.UsageSite()
	}
	counts := seqgrp.counts
	gappos := seqgrp.mapping[gapchar]

	nrow, ncol := counts.Size()
	total := make ([]float32, ncol) // total observations in each column
	for icol := 0; icol < ncol; icol++ {
		for irow := 0; irow < nrow; irow++ {
			total[icol] += counts.Mat[irow][icol]
		}
	}
	if gapsAreChar == false { // If necessary, remove gaps from the total
		for icol := 0; icol < ncol; icol++ { // Should benchmark and see
			if counts.Mat[gappos][icol] != 0.0 {  // if this if statement
				total[icol] -= counts.Mat[gappos][icol] // saves time.
			}
		}	
	}
	for icol := 0; icol < ncol; icol++ { // Normalise the counts
		for irow := 0; irow < nrow; irow++ {
			counts.Mat[irow][icol] /= (total[icol])
		}
	}	
	seqgrp.freqKnwn = true
}

// PrintFreqs prints out the frequencies of each character type
// It is probably only useful for debugging or testing.
// fmt is a format string like "%6.1f"
func (seqgrp *SeqGrp) PrintFreqs(format string) {
	if len(seqgrp.revmap) == 0 {
		seqgrp.UsageSite()
	}
	for ic, m := range seqgrp.revmap {
		fmt.Printf("%c ", m)
		for i := 0; i < len(seqgrp.seqs[0].seq); i++ {
			fmt.Printf(format, seqgrp.counts.Mat[ic][i])
		}
		fmt.Printf("\n")
	}
}

// Entropy calculates sequence entropy. It returns the result as a slice
// of the same length as the sequences. It needs to be told if gaps are
// characters, or should be ignored.
// If the sequence is a nucleotide or protein, we know what logarithm to use.
// If the sequence is unknown, we use the log base the number different
// symbols
func (seqgrp *SeqGrp) Entropy(gapsAreChar bool) ([]float32, error) {
	if !seqgrp.freqKnwn {
		seqgrp.UsageFrac(gapsAreChar)
	}
	var nSym float64
	const progbug = "program bug in Entropy"

	if gapsAreChar {
		switch seqgrp.GetType() {
		case DNA, RNA, Ntide:
			nSym = 5 // 4 nucleotides + gap character
		case Protein:
			nSym = 21 // 20 residues, + gap
		case Unknown:
			nSym = float64(len(seqgrp.revmap))
		default:
			return nil, fmt.Errorf(progbug)
		}
	} else { // gaps are ignored
		switch seqgrp.GetType() {
			case DNA, RNA, Ntide:
			nSym = 4
		case Protein:
			nSym = 20
		case Unknown:
			nSym = float64(len(seqgrp.revmap))
		default:
			return nil, fmt.Errorf(progbug)
		}
	}
	logfac := 1.0 / math.Log(nSym)
	entropy := make([]float32, len(seqgrp.seqs[0].GetSeq()))
	nrow, ncol := seqgrp.counts.Size()
	if gapsAreChar {
		for icol := 0; icol < ncol; icol++ {
			total := 0.0
			for irow := 0; irow < nrow; irow++ {
				f := float64(seqgrp.counts.Mat[irow][icol])
				if f == 0.0 {
					continue
				}
				logf := math.Log(f) * logfac
				total += f * logf
			}
			entropy[icol] = float32(math.Abs(total))
		}
	} else { // we have to check and ignore gap characters
		iBadRow := int(seqgrp.mapping['-'])
		fmt.Println ("badrow is ", iBadRow)
		for icol := 0; icol < ncol; icol++ {
			total := 0.0
			for irow := 0; irow < nrow; irow++ {
				if irow == iBadRow { continue }
				f := float64(seqgrp.counts.Mat[irow][icol])
				if f == 0.0 {
					continue
				}
				logf := math.Log(f) * logfac
				total += f * logf
			}
			entropy[icol] = float32(math.Abs(total))
		}

	}
	return entropy, nil
}
