package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// --- BitReader --- //
type BitReader struct {
	reader io.Reader
	buffer byte
	offset int // 0-7, number of bits already read from the buffer
}

func NewBitReader(r io.Reader) *BitReader {
	return &BitReader{reader: r}
}

func (br *BitReader) Read(n int) ([]byte, error) {
	bits := make([]byte, n)
	for i := 0; i < n; i++ {
		if br.offset == 0 || br.offset > 7 {
			buf := make([]byte, 1)
			_, err := br.reader.Read(buf)
			if err != nil {
				return bits[:i], err // Return bits read so far and the error
			}
			br.buffer = buf[0]
			br.offset = 0
		}

		bit := (br.buffer >> (7 - br.offset)) & 1
		bits[i] = bit
		br.offset++
	}
	return bits, nil
}

// --- BitWriter --- //
type BitWriter struct {
	writer *bufio.Writer
	buffer byte
	offset int // 0-7, number of bits written to the buffer
}

func NewBitWriter(w io.Writer) *BitWriter {
	return &BitWriter{writer: bufio.NewWriter(w)}
}

func (bw *BitWriter) Write(bits []byte) error {
	for _, bit := range bits {
		if bit == 1 {
			bw.buffer |= 1 << (7 - bw.offset)
		}
		bw.offset++
		if bw.offset == 8 {
			if err := bw.flushByte(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (bw *BitWriter) flushByte() error {
	if bw.offset == 0 {
		return nil
	}
	_, err := bw.writer.Write([]byte{bw.buffer})
	bw.buffer = 0
	bw.offset = 0
	return err
}

func (bw *BitWriter) Close() error {
	if err := bw.flushByte(); err != nil {
		return err
	}
	return bw.writer.Flush()
}

// --- Main Logic --- 

func main() {
	patternStr := flag.String("p", "", "Permutation pattern (e.g., \"1,0\"). Enables Permute Mode.")
	elementSize := flag.Int("s", 0, "(Required) Size of each element in bits.")
	inverse := flag.Bool("inverse", false, "Apply the inverse of the pattern (in Permute Mode).")
	splitN := flag.Int("split", 0, "Number of output streams. Enables De-mux Mode.")
	inputFile := flag.String("i", "", "Input file path (for Permute and De-mux modes).")
	outputFile := flag.String("o", "", "Output file path (for Permute and Mux modes).")
	flag.Parse()

	muxInputFiles := flag.Args()

	if *elementSize <= 0 {
		fmt.Fprintln(os.Stderr, "Error: -s <size> is a required flag and must be > 0.")
		os.Exit(1)
	}

	if *patternStr != "" {
		if len(muxInputFiles) > 0 || *splitN > 0 {
			fmt.Fprintln(os.Stderr, "Error: -p (Permute Mode) cannot be used with multiple input files or --split.")
			os.Exit(1)
		}
		if err := runPermuteMode(*inputFile, *outputFile, *patternStr, *elementSize, *inverse); err != nil {
			fmt.Fprintf(os.Stderr, "Error in Permute Mode: %v\n", err)
			os.Exit(1)
		}
	} else if len(muxInputFiles) > 0 {
		if *splitN > 0 {
			fmt.Fprintln(os.Stderr, "Error: Cannot combine multiple input files and use --split at the same time.")
			os.Exit(1)
		}
		if *outputFile == "" {
			fmt.Fprintln(os.Stderr, "Error: -o <output_file> is required when providing multiple input files (Mux Mode).")
			os.Exit(1)
		}
		if err := runMuxMode(muxInputFiles, *outputFile, *elementSize); err != nil {
			fmt.Fprintf(os.Stderr, "Error in Mux Mode: %v\n", err)
			os.Exit(1)
		}
	} else if *splitN > 0 {
		if *inputFile == "" {
			fmt.Fprintln(os.Stderr, "Error: -i <input_file> is required when using --split (De-mux Mode).")
			os.Exit(1)
		}
		if err := runDeMuxMode(*inputFile, *splitN, *elementSize); err != nil {
			fmt.Fprintf(os.Stderr, "Error in De-mux Mode: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Error: Invalid combination of flags. Please specify a mode.")
		os.Exit(1)
	}
}

// --- Mode 1: Permute (Unchanged) --- 
func runPermuteMode(inputFile, outputFile, patternStr string, elementSize int, inverse bool) error {
	var reader io.Reader = os.Stdin
	if inputFile != "" && inputFile != "-" {
		file, err := os.Open(inputFile)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}

	var writer io.Writer = os.Stdout
	if outputFile != "" && outputFile != "-" {
		file, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = bufio.NewWriter(file)
		defer writer.(*bufio.Writer).Flush()
	}

	inputData, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	outputData, err := processInterleave(inputData, patternStr, elementSize, inverse)
	if err != nil {
		return err
	}

	_, err = writer.Write(outputData)
	return err
}

// --- Mode 2: Mux (Rewritten for bit-level operations) --- 
func runMuxMode(inputFilePaths []string, outputFilePath string, elementSize int) error {
	readers := make([]*os.File, len(inputFilePaths))
	for i, path := range inputFilePaths {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		readers[i] = file
		defer file.Close()
	}

	bitReaders := make([]*BitReader, len(readers))
	for i, r := range readers {
		bitReaders[i] = NewBitReader(bufio.NewReader(r))
	}

	outFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	bitWriter := NewBitWriter(outFile)

	for {
		filesAtEOF := 0
		for _, br := range bitReaders {
			bits, err := br.Read(elementSize)
			if len(bits) > 0 {
				if wErr := bitWriter.Write(bits); wErr != nil {
					return wErr
				}
			}
			if err != nil {
				filesAtEOF++
			}
		}
		if filesAtEOF >= len(bitReaders) {
			break
		}
	}
	return bitWriter.Close()
}

// --- Mode 3: De-mux (Rewritten for bit-level operations) --- 
func runDeMuxMode(inputFilePath string, numStreams, elementSize int) error {
	inFile, err := os.Open(inputFilePath)
	if err != nil {
		return err
	}
	defer inFile.Close()
	bitReader := NewBitReader(bufio.NewReader(inFile))

	outFiles := make([]*os.File, numStreams)
	bitWriters := make([]*BitWriter, numStreams)
	for i := 0; i < numStreams; i++ {
		outputName := generateSplitFileName(inputFilePath, i)
		outFile, err := os.Create(outputName)
		if err != nil {
			return err
		}
		outFiles[i] = outFile // Keep track to close it properly
		bitWriters[i] = NewBitWriter(outFile)
	}

	// Defer closing the file handles
	for _, f := range outFiles {
		defer f.Close()
	}

	streamIndex := 0
	for {
		bits, err := bitReader.Read(elementSize)
		if len(bits) > 0 {
			if wErr := bitWriters[streamIndex].Write(bits); wErr != nil {
				return wErr
			}
		}
		if err != nil {
			break // EOF or other error
		}
		streamIndex = (streamIndex + 1) % numStreams
	}

	// Explicitly close/flush all bit writers
	for _, bw := range bitWriters {
		if err := bw.Close(); err != nil {
			return err
		}
	}
	return nil
}

// --- Helpers --- 

func generateSplitFileName(originalPath string, index int) string {
	ext := filepath.Ext(originalPath)
	base := strings.TrimSuffix(originalPath, ext)
	return fmt.Sprintf("%s_%d%s", base, index, ext)
}

func processInterleave(data []byte, patternStr string, elementSize int, inverse bool) ([]byte, error) {
	pattern, err := parsePattern(patternStr)
	if err != nil {
		return nil, err
	}
	if inverse {
		pattern = invertPattern(pattern)
	}

	inputBits := bytesToBits(data)
	outputBits := new(bytes.Buffer)
	blockSize := len(pattern)
	blockSizeInBits := blockSize * elementSize

	for i := 0; i < len(inputBits); i += blockSizeInBits {
		end := i + blockSizeInBits
		if end > len(inputBits) {
			end = len(inputBits)
		}
		inputChunk := inputBits[i:end]
		numElementsInChunk := len(inputChunk) / elementSize

		if numElementsInChunk == blockSize {
			permutedChunk := make([]byte, blockSizeInBits)
			for j := 0; j < blockSize; j++ {
				sourceIndex := pattern[j]
				copy(permutedChunk[j*elementSize:(j+1)*elementSize], inputChunk[sourceIndex*elementSize:(sourceIndex+1)*elementSize])
			}
			outputBits.Write(permutedChunk)
		} else {
			outputBits.Write(inputChunk)
		}
	}
	return bitsToBytes(outputBits.Bytes()), nil
}

func parsePattern(patternStr string) ([]int, error) {
	parts := strings.Split(patternStr, ",")
	pattern := make([]int, len(parts))
	for i, p := range parts {
		val, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: contains non-integer value '%s'", p)
		}
		pattern[i] = val
	}
	if !isPermutation(pattern) {
		return nil, fmt.Errorf("invalid pattern: must be a valid permutation of 0..N-1")
	}
	return pattern, nil
}

func isPermutation(p []int) bool {
	n := len(p)
	seen := make(map[int]bool, n)
	for _, val := range p {
		if val < 0 || val >= n || seen[val] {
			return false
		}
		seen[val] = true
	}
	return true
}

func invertPattern(pattern []int) []int {
	n := len(pattern)
	inverse := make([]int, n)
	for i, p := range pattern {
		inverse[p] = i
	}
	return inverse
}

func bytesToBits(data []byte) []byte {
	bits := make([]byte, len(data)*8)
	for i, b := range data {
		for j := 0; j < 8; j++ {
			if (b>>(7-j))&1 == 1 {
				bits[i*8+j] = 1
			} else {
				bits[i*8+j] = 0
			}
		}
	}
	return bits
}

func bitsToBytes(bits []byte) []byte {
	byteCount := (len(bits) + 7) / 8
	data := make([]byte, byteCount)
	for i := 0; i < len(bits); i++ {
		if bits[i] == 1 {
			byteIndex := i / 8
			bitIndex := i % 8
			data[byteIndex] |= 1 << (7 - bitIndex)
		}
	}
	return data
}
