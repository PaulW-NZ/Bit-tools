package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

func main() {
	encodeMode := flag.Bool("encode", false, "Encode data with Hamming code")
	decodeMode := flag.Bool("decode", false, "Decode Hamming coded data and correct errors")
	mFlag := flag.Int("m", 3, "Parameter m for Hamming code, defines (2^m-1, 2^m-1-m) code")
	extended := flag.Bool("extended", false, "Use extended Hamming code")
	verbose := flag.Bool("v", false, "Verbose mode: print error correction details to stderr")
	inFile := flag.String("i", "", "Input file (defaults to stdin)")
	outFile := flag.String("o", "", "Output file (defaults to stdout)")

	flag.Parse()

	if *encodeMode == *decodeMode {
		log.Fatal("Error: You must specify exactly one of -encode or -decode modes.")
	}

	var inputData []byte
	var err error
	if *inFile == "" {
		inputData, err = ioutil.ReadAll(os.Stdin)
	} else {
		inputData, err = ioutil.ReadFile(*inFile)
	}
	if err != nil {
		log.Fatalf("Failed to read input: %s", err)
	}

	var outputData []byte

	if *encodeMode {
		outputData = encode(inputData, *mFlag, *extended)
	} else {
		outputData = decode(inputData, *mFlag, *extended, *verbose)
	}

	if *outFile == "" {
		_, err = os.Stdout.Write(outputData)
	} else {
		err = ioutil.WriteFile(*outFile, outputData, 0644)
	}
	if err != nil {
		log.Fatalf("Failed to write output: %s", err)
	}
}

func encode(data []byte, m int, extended bool) []byte {
	k := (1 << m) - 1 - m
	reader := newBitReader(data)
	writer := newBitWriter()

	size := uint64(len(data))
	for i := 0; i < 64; i++ {
		bit := (size >> (63 - uint(i))) & 1
		writer.Write(uint(bit), 1)
	}

	for {
		dataBits := make([]uint, k)
		firstBit, err := reader.Read(1)
		if err != nil {
			break
		}
		dataBits[0] = firstBit
		for i := 1; i < k; i++ {
			bit, _ := reader.Read(1)
			dataBits[i] = bit
		}

	hammingBlock := encodeBlock(dataBits, m)

		if extended {
			overallParity := uint(0)
			for _, bit := range hammingBlock {
				overallParity ^= bit
			}
			writer.Write(overallParity, 1)
		}

		for _, bit := range hammingBlock {
			writer.Write(bit, 1)
		}
	}
	return writer.Bytes()
}

func encodeBlock(dataBits []uint, m int) []uint {
	n := (1 << m) - 1
	block := make([]uint, n)
	dataBitIndex := 0
	for i := 1; i <= n; i++ {
		if (i > 0) && ((i & (i - 1)) == 0) {
			// parity bit position
		} else {
			block[i-1] = dataBits[dataBitIndex]
			dataBitIndex++
		}
	}

	for i := 0; i < m; i++ {
		pPos := 1 << i
		parity := uint(0)
		for j := 1; j <= n; j++ {
			if j != pPos && (j&pPos != 0) {
				parity ^= block[j-1]
			}
		}
		block[pPos-1] = parity
	}
	return block
}

func decode(data []byte, m int, extended bool, verbose bool) []byte {
	n_orig := (1 << m) - 1
	n := n_orig
	if extended {
		n++
	}
	reader := newBitReader(data)

	var size uint64
	for i := 0; i < 64; i++ {
		bit, err := reader.Read(1)
		if err != nil {
			log.Fatal("Failed to read size from input file")
		}
		size = (size << 1) | uint64(bit)
	}

	writer := newBitWriter()
	blockNum := 0

	for {
		block := make([]uint, n)
		readCount := 0
		for i := 0; i < n; i++ {
			bit, err := reader.Read(1)
			if err != nil {
				break
			}
			block[i] = bit
			readCount++
		}

		if readCount < n {
			break
		}

		dataBits := decodeBlock(block, m, extended, verbose, blockNum)

		for _, bit := range dataBits {
			writer.Write(bit, 1)
		}
		blockNum++
	}

	decodedData := writer.Bytes()
	if uint64(len(decodedData)) > size {
		return decodedData[:size]
	}
	return decodedData
}

func decodeBlock(block []uint, m int, extended bool, verbose bool, blockNum int) []uint {
	n_orig := (1 << m) - 1
	hammingBlock := block

	if extended {
		overallParityBit := block[0]
		hammingBlock = block[1:]
		overallParity := uint(0)
		for _, bit := range hammingBlock {
			overallParity ^= bit
		}

		syndrome := calculateSyndrome(hammingBlock, m)

		if overallParity != overallParityBit {
			if syndrome != 0 {
				if syndrome-1 < len(hammingBlock) {
					hammingBlock[syndrome-1] ^= 1
					if verbose {
						fmt.Fprintf(os.Stderr, "Corrected 1-bit error in block %d at position %d\n", blockNum, syndrome)
					}
				}
			}
		} else if syndrome != 0 {
			fmt.Fprintf(os.Stderr, "Warning: Uncorrectable 2-bit error detected in block %d\n", blockNum)
		}
	} else {
		syndrome := calculateSyndrome(hammingBlock, m)
		if syndrome != 0 {
			if syndrome-1 < len(hammingBlock) {
				hammingBlock[syndrome-1] ^= 1
				if verbose {
					fmt.Fprintf(os.Stderr, "Corrected 1-bit error in block %d at position %d\n", blockNum, syndrome)
				}
			}
		}
	}

	dataBits := make([]uint, 0, n_orig-m)
	for i := 1; i <= len(hammingBlock); i++ {
		if (i > 0) && ((i & (i - 1)) != 0) {
			dataBits = append(dataBits, hammingBlock[i-1])
		}
	}
	return dataBits
}

func calculateSyndrome(block []uint, m int) int {
	n := (1 << m) - 1
	syndrome := 0
	for i := 0; i < m; i++ {
		pPos := 1 << i
		parity := uint(0)
		for j := 1; j <= n; j++ {
			if j&pPos != 0 {
				if j-1 < len(block) {
					parity ^= block[j-1]
				}
			}
		}
		if parity != 0 {
			syndrome += pPos
		}
	}
	return syndrome
}

// --- Bit-level Helpers ---

type bitReader struct {
	data []byte
	byte int
	bit  uint
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (r *bitReader) Read(n int) (uint, error) {
	if r.byte >= len(r.data) {
		return 0, fmt.Errorf("end of data")
	}
	val := (uint(r.data[r.byte]) >> (7 - r.bit)) & 1
	r.bit++
	if r.bit == 8 {
		r.bit = 0
		r.byte++
	}
	return val, nil
}

type bitWriter struct {
	data []byte
	byte byte
	bit  uint
}

func newBitWriter() *bitWriter {
	return &bitWriter{}
}

func (w *bitWriter) Write(val uint, n int) {
	for i := 0; i < n; i++ {
		bit := (val >> uint(n-1-i)) & 1
		w.byte |= byte(bit << (7 - w.bit))
		w.bit++
		if w.bit == 8 {
			w.data = append(w.data, w.byte)
			w.byte = 0
			w.bit = 0
		}
	}
}

func (w *bitWriter) Bytes() []byte {
	if w.bit > 0 {
		w.data = append(w.data, w.byte)
	}
	return w.data
}