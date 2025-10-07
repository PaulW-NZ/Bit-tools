package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// --- BitReader ---

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
				return bits[:i], err
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

// --- BitWriter ---

type BitWriter struct {
	writer io.Writer
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
	return bw.writer.(*bufio.Writer).Flush()
}

// --- Main Logic ---

func main() {
	mode := flag.String("mode", "gen", "Operating mode: gen, cipher, scramble, descramble")
	polyStr := flag.String("p", "", "(Required) Polynomial taps, comma-separated (e.g., \"16,14,13,11\")")
	seedStr := flag.String("s", "", "Initial fill/seed as a binary string (for gen and cipher modes).")
	numBits := flag.Int64("n", 0, "Number of bits to generate (in gen mode).")
	inputFile := flag.String("i", "", "Input file path (for cipher, scramble, and descramble modes).")
	outputFile := flag.String("o", "", "Output file path.")
	flag.Parse()

	switch *mode {
	case "gen":
		if err := runGenMode(*polyStr, *seedStr, *numBits, *outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error in gen mode: %v\n", err)
			os.Exit(1)
		}
	case "cipher":
		if err := runCipherMode(*polyStr, *seedStr, *inputFile, *outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error in cipher mode: %v\n", err)
			os.Exit(1)
		}
	case "scramble":
		if err := runScrambleMode(*polyStr, *inputFile, *outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error in scramble mode: %v\n", err)
			os.Exit(1)
		}
	case "descramble":
		if err := runDescrambleMode(*polyStr, *inputFile, *outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error in descramble mode: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown mode '%s'. Valid modes are: gen, cipher, scramble, descramble.\n", *mode)
		os.Exit(1)
	}
}

// --- Mode 1: Generate Sequence ---
func runGenMode(polyStr, seedStr string, numBits int64, outputFilePath string) error {
	if polyStr == "" || seedStr == "" || numBits <= 0 {
		return errors.New("-p, -s, and -n are required for gen mode")
	}

	poly, degree, err := parsePoly(polyStr)
	if err != nil {
		return err
	}

	state, err := parseSeed(seedStr)
	if err != nil {
		return err
	}

	if len(state) != degree {
		return fmt.Errorf("seed length (%d) must match the polynomial degree (%d)", len(state), degree)
	}

	var writer io.Writer = os.Stdout
	if outputFilePath != "" && outputFilePath != "-" {
		file, err := os.Create(outputFilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	bitWriter := NewBitWriter(writer)

	for i := int64(0); i < numBits; i++ {
		outputBit := state[degree-1]
		if err := bitWriter.Write([]byte{outputBit}); err != nil {
			return err
		}

		feedbackBit := byte(0)
		for _, tap := range poly {
			feedbackBit ^= state[tap-1]
		}

		copy(state[1:], state[:degree-1])
		state[0] = feedbackBit
	}

	return bitWriter.Close()
}

// --- Mode 2: Stream Cipher ---
func runCipherMode(polyStr, seedStr, inputFilePath, outputFilePath string) error {
	if polyStr == "" || seedStr == "" {
		return errors.New("-p and -s are required for cipher mode")
	}

	poly, degree, err := parsePoly(polyStr)
	if err != nil {
		return err
	}

	state, err := parseSeed(seedStr)
	if err != nil {
		return err
	}

	if len(state) != degree {
		return fmt.Errorf("seed length (%d) must match the polynomial degree (%d)", len(state), degree)
	}

	var reader io.Reader = os.Stdin
	if inputFilePath != "" && inputFilePath != "-" {
		file, err := os.Open(inputFilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	bitReader := NewBitReader(reader)

	var writer io.Writer = os.Stdout
	if outputFilePath != "" && outputFilePath != "-" {
		file, err := os.Create(outputFilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	bitWriter := NewBitWriter(writer)

	for {
		dataBitSlice, err := bitReader.Read(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if len(dataBitSlice) == 0 {
			break
		}
		dataBit := dataBitSlice[0]

		keystreamBit := state[degree-1]

		feedbackBit := byte(0)
		for _, tap := range poly {
			feedbackBit ^= state[tap-1]
		}
		copy(state[1:], state[:degree-1])
		state[0] = feedbackBit

		outputBit := dataBit ^ keystreamBit

		if err := bitWriter.Write([]byte{outputBit}); err != nil {
			return err
		}
	}

	return bitWriter.Close()
}

// --- Mode 3: Feed-Through Scrambler ---
func runScrambleMode(polyStr, inputFilePath, outputFilePath string) error {
	if polyStr == "" {
		return errors.New("-p is required for scramble mode")
	}

	poly, degree, err := parsePoly(polyStr)
	if err != nil {
		return err
	}

	// Scrambler state is initialized to all zeros
	state := make([]byte, degree)

	var reader io.Reader = os.Stdin
	if inputFilePath != "" && inputFilePath != "-" {
		file, err := os.Open(inputFilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	bitReader := NewBitReader(reader)

	var writer io.Writer = os.Stdout
	if outputFilePath != "" && outputFilePath != "-" {
		file, err := os.Create(outputFilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	bitWriter := NewBitWriter(writer)

	for {
		dataBitSlice, err := bitReader.Read(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if len(dataBitSlice) == 0 {
			break
		}
		dataBit := dataBitSlice[0]

		// 1. Calculate feedback from current state
		feedbackBit := byte(0)
		for _, tap := range poly {
			feedbackBit ^= state[tap-1]
		}

		// 2. XOR data with feedback to create the output bit
		outputBit := dataBit ^ feedbackBit

		// 3. Shift register
		copy(state[1:], state[:degree-1])

		// 4. Set new input bit, which is the scrambled output bit
		state[0] = outputBit // LFSR is fed by its own output

		// 5. Write the result
		if err := bitWriter.Write([]byte{outputBit}); err != nil {
			return err
		}
	}

	return bitWriter.Close()
}

// --- Mode 4: Feed-Through Descrambler ---
func runDescrambleMode(polyStr, inputFilePath, outputFilePath string) error {
	if polyStr == "" {
		return errors.New("-p is required for descramble mode")
	}

	poly, degree, err := parsePoly(polyStr)
	if err != nil {
		return err
	}

	// Descrambler state is initialized to all zeros
	state := make([]byte, degree)

	var reader io.Reader = os.Stdin
	if inputFilePath != "" && inputFilePath != "-" {
		file, err := os.Open(inputFilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	bitReader := NewBitReader(reader)

	var writer io.Writer = os.Stdout
	if outputFilePath != "" && outputFilePath != "-" {
		file, err := os.Create(outputFilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	bitWriter := NewBitWriter(writer)

	for {
		dataBitSlice, err := bitReader.Read(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if len(dataBitSlice) == 0 {
			break
		}
		dataBit := dataBitSlice[0]

		// 1. Calculate feedback from current state
		feedbackBit := byte(0)
		for _, tap := range poly {
			feedbackBit ^= state[tap-1]
		}

		// 2. XOR data with feedback to create the output bit (descrambled data)
		outputBit := dataBit ^ feedbackBit

		// 3. Shift register
		copy(state[1:], state[:degree-1])

		// 4. Set new input bit, which is the *input* to the descrambler (scrambled data)
		state[0] = dataBit // LFSR is fed by the scrambled input

		// 5. Write the result
		if err := bitWriter.Write([]byte{outputBit}); err != nil {
			return err
		}
	}

	return bitWriter.Close()
}

// --- Helper Functions ---

func parsePoly(polyStr string) (taps []int, degree int, err error) {
	parts := strings.Split(polyStr, ",")
	if len(parts) == 0 {
		return nil, 0, errors.New("polynomial cannot be empty")
	}

	for _, p := range parts {
		tap, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, 0, fmt.Errorf("invalid tap value: %s", p)
		}
		if tap <= 0 {
			return nil, 0, fmt.Errorf("tap values must be positive: %d", tap)
		}
		taps = append(taps, tap)
	}

	degree = 0
	for _, tap := range taps {
		if tap > degree {
			degree = tap
		}
	}

	return taps, degree, nil
}

func parseSeed(seedStr string) ([]byte, error) {
	seed := make([]byte, len(seedStr))
	for i, char := range seedStr {
		if char == '1' {
			seed[i] = 1
		} else if char == '0' {
			seed[i] = 0
		} else {
			return nil, fmt.Errorf("invalid character in seed string: %c", char)
		}
	}
	return seed, nil
}
