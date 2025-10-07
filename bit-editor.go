package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var commandNames = map[rune]string{
	't': "Take",
	's': "Skip",
	'i': "Insert",
	'n': "Invert",
	'v': "Reverse Bits",
	'b': "Byte-Swap",
	'x': "XOR",
	'a': "AND",
	'o': "OR",
}

func printHelp() {
	fmt.Println(`Bit Editor - A command-line tool for bit-level file manipulation.`)
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  ./bit-editor -e \"<commands>\" [-i <in_file>] [-o <out_file>] [--start <bit>] [--end <bit>]")
	fmt.Println("  cat <in_file> | ./bit-editor -e \"<commands>\" > <out_file>")
	fmt.Println()
	fmt.Println("FLAGS:")
	fmt.Println("  -e string")
	fmt.Println("    \t(Required) The repeating string of edit commands.")
	fmt.Println("  -i string")
	fmt.Println("    \tInput file path. Defaults to standard input.")
	fmt.Println("  -o string")
	fmt.Println("    \tOutput file path. Defaults to standard output.")
	fmt.Println("  --start int")
	fmt.Println("    \tThe bit position to start editing from (inclusive). Defaults to 0.")
	fmt.Println("  --end int")
	fmt.Println("    \tThe bit position to stop editing at (exclusive). Defaults to the end of the data.")
	fmt.Println("  --verbose")
	fmt.Println("    \tEnable verbose logging for every loop of the command sequence.")
	fmt.Println("  --verbose-once")
	fmt.Println("    \tEnable verbose logging for the first command sequence loop only.")
	fmt.Println("  --dry-run")
	fmt.Println("    \tSimulate operations and report output size without writing data.")
	fmt.Println("  --help")
	fmt.Println("    \tShow this detailed help message.")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  --- Stream Operations ---")
	fmt.Println("  t<number>    Take <number> bits from the input stream.")
	fmt.Println("  s<number>    Skip <number> bits from the input stream.")
	fmt.Println("  i<binary>    Insert a literal <binary> string into the output.")
	fmt.Println("  n<number>    Invert the next <number> bits from the input stream.")
	fmt.Println()
	fmt.Println("  --- Re-ordering Operations ---")
	fmt.Println("  v<number>    Reverse the order of BITS within the next <number>-bit word.")
	fmt.Println("  b<number>    Reverse the order of BYTES within the next <number>-bit word (for endian swapping).")
	fmt.Println()
	fmt.Println("  --- Logical Operations ---")
	fmt.Println("  x<N>:<P>    XOR the next <N> bits with the repeating pattern <P>.")
	fmt.Println("  a<N>:<P>    AND the next <N> bits with the repeating pattern <P>.")
	fmt.Println("  o<N>:<P>    OR the next <N> bits with the repeating pattern <P>.")
	fmt.Println()
	fmt.Println("  --- Block Operations ---")
	fmt.Println("  [<chain>]<N>  Processes the next <N> bits as a single block, applying the <chain> of commands to it.")
	fmt.Println("               - Allowed commands in a chain: n, v, b, x, a, o.")
	fmt.Println("               - Commands inside a block apply to the whole block (e.g., 'n' inverts all N bits).")
	fmt.Println("               - Logical ops in a chain still require a pattern (e.g., [nx:101]8).")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  1. Extract 1 byte from every 3 bytes:")
	fmt.Println("     ./bit-editor -e \"s16t8\" -i in.dat -o out.dat")
	fmt.Println()
	fmt.Println("  2. Change endianness of a file with 32-bit (4-byte) words:")
	fmt.Println("     ./bit-editor -e \"b32\" -i in.dat -o out.dat")
	fmt.Println()
	fmt.Println("  3. Reverse and Invert each byte of a file (with verbose logging):")
	fmt.Println("     ./bit-editor -e \"[vn]8\" --verbose -i in.dat -o out.dat")
	fmt.Println()
	fmt.Println("  4. Check the output size of a complex operation without writing the file:")
	fmt.Println("     ./bit-editor -e \"[a:11110000]16[b]16\" --dry-run -i in.dat")
}

func main() {
	// 1. Define and parse command-line flags
	detailedHelp := flag.Bool("help", false, "Show detailed help text and examples.")
	verbose := flag.Bool("verbose", false, "Enable verbose logging for every loop of the command sequence.")
	verboseOnce := flag.Bool("verbose-once", false, "Enable verbose logging for the first command sequence loop only.")
	dryRun := flag.Bool("dry-run", false, "Simulate operations and report output size without writing data.")
	inputFile := flag.String("i", "", "Input file path. Defaults to stdin.")
	outputFile := flag.String("o", "", "Output file path. Defaults to stdout.")
	editString := flag.String("e", "", "Edit command string (e.g., 's16t8'). Required.")
	startBit := flag.Int("start", 0, "Start bit for editing (inclusive).")
	endBit := flag.Int("end", 0, "End bit for editing (exclusive). Defaults to the end of the data.")
	flag.Parse()

	if *detailedHelp {
		printHelp()
		os.Exit(0)
	}

	if *editString == "" {
		fmt.Fprintln(os.Stderr, "Error: -e <editString> is required.")
		flag.Usage()
		os.Exit(1)
	}

	// 2. Set up input reader
	var reader io.Reader
	if *inputFile == "" || *inputFile == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		reader = file
	}

	// 4. Read input data
	inputData, err := io.ReadAll(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// 5. Apply edits
	isVerbose := *verbose || *verboseOnce
	outputData, err := applyEdits(inputData, *editString, *startBit, *endBit, isVerbose, *verboseOnce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error applying edits: %v\n", err)
		os.Exit(1)
	}

	// 6. Write output data or print dry run summary
	if *dryRun {
		fmt.Printf("Dry run complete. Output would be %d bytes.\n", len(outputData))
	} else {
		var writer io.Writer
		if *outputFile == "" || *outputFile == "-" {
			writer = os.Stdout
		} else {
			file, err := os.Create(*outputFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
				os.Exit(1)
			}
			defer file.Close()
			writer = bufio.NewWriter(file)
			defer writer.(*bufio.Writer).Flush()
		}
		_, err = writer.Write(outputData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		}
	}
}

// bytesToBits converts a slice of bytes to a slice of bits (0s and 1s).
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

// bitsToBytes converts a slice of bits (0s and 1s) to a slice of bytes.
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

// applyBlockOps applies a series of transformations to a single chunk of bits.
func applyBlockOps(initialChunk []byte, subProgram string, verbose bool) ([]byte, error) {
	processedChunk := make([]byte, len(initialChunk))
	copy(processedChunk, initialChunk)

	cmdIdx := 0
	for cmdIdx < len(subProgram) {
		command := rune(subProgram[cmdIdx])
		cmdIdx++

		argStr := ""
		if strings.ContainsRune("xao", command) {
			nextCmdIdx := len(subProgram)
			for i := cmdIdx; i < len(subProgram); i++ {
				if strings.ContainsRune("nvxao", rune(subProgram[i])) {
					nextCmdIdx = i
					break
				}
			}
			argStr = subProgram[cmdIdx:nextCmdIdx]
			cmdIdx = nextCmdIdx
		}

		if verbose {
			logArg := argStr
			if logArg != "" {
				logArg = " with arg \"" + logArg + "\""
			}
			fmt.Fprintf(os.Stderr, "    -> Applying block command '%s'%s\n", commandNames[command], logArg)
		}

		switch command {
		case 'n':
			for i, bit := range processedChunk {
				processedChunk[i] = 1 - bit
			}
		case 'v':
			for i, j := 0, len(processedChunk)-1; i < j; i, j = i+1, j-1 {
				processedChunk[i], processedChunk[j] = processedChunk[j], processedChunk[i]
			}
		case 'b':
			numBytes := len(processedChunk) / 8
			if numBytes > 1 {
				tempChunk := make([]byte, len(processedChunk))
				copy(tempChunk, processedChunk)
				for i := 0; i < numBytes; i++ {
					destByteStart := i * 8
				sourceByteIndex := numBytes - 1 - i
				sourceByteStart := sourceByteIndex * 8
				copy(processedChunk[destByteStart:destByteStart+8], tempChunk[sourceByteStart:sourceByteStart+8])
				}
			}
		case 'x', 'a', 'o':
			if !strings.Contains(argStr, ":") {
				return nil, fmt.Errorf("logical op '%c' in block requires a pattern (e.g., x:101)", command)
			}
			parts := strings.SplitN(argStr, ":", 2)
			pattern := parts[1]
			if len(pattern) == 0 {
				return nil, fmt.Errorf("pattern for '%c' cannot be empty", command)
			}
			for i, bit := range processedChunk {
				patternBit := byte(pattern[i%len(pattern)] - '0')
				var resultBit byte
				switch command {
				case 'x':
					resultBit = bit ^ patternBit
				case 'a':
					resultBit = bit & patternBit
				case 'o':
					resultBit = bit | patternBit
				}
				processedChunk[i] = resultBit
			}
		case 't', 's', 'i':
			return nil, fmt.Errorf("command '%c' not allowed in block operation", command)
			default:
			return nil, fmt.Errorf("unknown command '%c' in block operation", command)
		}
	}
	return processedChunk, nil
}

// applyEdits processes the input data according to the repeating edit command string.
func applyEdits(data []byte, commands string, startBit, endBit int, verbose, verboseOnce bool) ([]byte, error) {

	inputBits := bytesToBits(data)
	outputBits := new(bytes.Buffer)

	// Validate and adjust start/end bits
	if startBit < 0 || startBit > len(inputBits) {
		return nil, fmt.Errorf("start bit (%d) is out of bounds", startBit)
	}
	if endBit <= 0 || endBit > len(inputBits) {
		endBit = len(inputBits)
	}
	if startBit > endBit {
		return nil, fmt.Errorf("start bit (%d) cannot be greater than end bit (%d)", startBit, endBit)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Starting edit process. Total input bits: %d. Processing range: %d to %d.\n", len(inputBits), startBit, endBit)
	}

	inputPos := startBit
	logPrinted := false

	// Main loop to repeat the command pattern until the end of the specified range
	for inputPos < endBit {
		if len(commands) == 0 {
			break
		}

		cmdIdx := 0
		for cmdIdx < len(commands) {
			if inputPos >= endBit {
				break
			}

			command := rune(commands[cmdIdx])
			bitsBefore := outputBits.Len()
			shouldLog := verbose && (!verboseOnce || !logPrinted)

			if command == '[' {
				cmdIdx++ // Move past '['
				endBracketIdx := strings.IndexRune(commands[cmdIdx:], ']')
				if endBracketIdx == -1 {
					return nil, fmt.Errorf("mismatched brackets in command string")
				}
			endBracketIdx += cmdIdx
			subProgram := commands[cmdIdx:endBracketIdx]

			numStartIdx := endBracketIdx + 1
			numEndIdx := numStartIdx
			for numEndIdx < len(commands) && commands[numEndIdx] >= '0' && commands[numEndIdx] <= '9' {
				numEndIdx++
			}

			if numStartIdx == numEndIdx {
				return nil, fmt.Errorf("block operation must be followed by a number")
			}

			count, err := strconv.Atoi(commands[numStartIdx:numEndIdx])
			if err != nil {
				return nil, fmt.Errorf("invalid number for block operation: %s", commands[numStartIdx:numEndIdx])
			}

			if shouldLog {
				fmt.Fprintf(os.Stderr, "Processing block command \"[%s]%d\" at input bit %d\n", subProgram, count, inputPos)
			}

			readEnd := inputPos + count
			if readEnd > endBit {
				readEnd = endBit
			}

			chunk := inputBits[inputPos:readEnd]
			processedChunk, err := applyBlockOps(chunk, subProgram, shouldLog)
			if err != nil {
				return nil, err
			}

			outputBits.Write(processedChunk)
			inputPos = readEnd
			cmdIdx = numEndIdx

			if shouldLog {
				bitsAfter := outputBits.Len()
				fmt.Fprintf(os.Stderr, " -> Wrote %d bits to output.\n", bitsAfter-bitsBefore)
			}
			continue
		}

		cmdIdx++
		// --- Argument Parsing for simple commands ---
		argStart := cmdIdx
		argEnd := cmdIdx
		nextCmdIdx := len(commands)
		for i := cmdIdx; i < len(commands); i++ {
			if strings.ContainsRune("tsnivxaob[", rune(commands[i])) {
				nextCmdIdx = i
				break
			}
		}
		argEnd = nextCmdIdx
		argStr := commands[argStart:argEnd]
		cmdIdx = argEnd
		// --- End Argument Parsing ---

		if shouldLog {
			fmt.Fprintf(os.Stderr, "Processing '%s' command with arg \"%s\" at input bit %d\n", commandNames[command], argStr, inputPos)
		}

		switch command {
		case 't', 's', 'n', 'v', 'b':
			count, err := strconv.Atoi(argStr)
			if err != nil {
				return nil, fmt.Errorf("invalid numeric argument for command '%c': %s", command, argStr)
			}

			switch command {
			case 't':
				readEnd := inputPos + count
				if readEnd > endBit {
					readEnd = endBit
				}
				outputBits.Write(inputBits[inputPos:readEnd])
				inputPos = readEnd
			case 's':
				inputPos += count
			case 'n':
				readEnd := inputPos + count
				if readEnd > endBit {
					readEnd = endBit
				}
				for _, bit := range inputBits[inputPos:readEnd] {
					outputBits.WriteByte(1 - bit)
				}
				inputPos = readEnd
			case 'v':
				readEnd := inputPos + count
				if readEnd > endBit {
					readEnd = endBit
				}
				chunk := inputBits[inputPos:readEnd]
				for i := len(chunk) - 1; i >= 0; i-- {
					outputBits.WriteByte(chunk[i])
				}
				inputPos = readEnd
			case 'b':
				if count%8 != 0 {
					return nil, fmt.Errorf("argument for 'b' command must be a multiple of 8, got %d", count)
				}
				readEnd := inputPos + count
				if readEnd > endBit {
					readEnd = endBit
				}
				chunk := inputBits[inputPos:readEnd]
				numBytes := len(chunk) / 8
				if numBytes > 0 {
					for i := numBytes - 1; i >= 0; i-- {
						byteStart := i * 8
						outputBits.Write(chunk[byteStart : byteStart+8])
					}
				}
				// Write any remaining bits that don't form a full byte
				if len(chunk)%8 != 0 {
					outputBits.Write(chunk[numBytes*8:])
				}
				inputPos = readEnd
			}

		case 'i':
			for _, char := range argStr {
				if char != '0' && char != '1' {
					return nil, fmt.Errorf("invalid binary string for 'i' command: %s", argStr)
				}
				outputBits.WriteByte(byte(char - '0'))
			}

		case 'x', 'a', 'o':
			parts := strings.SplitN(argStr, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid argument for command '%c': expected <number>:<pattern>, got %s", command, argStr)
			}

			count, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid numeric count for command '%c': %s", command, parts[0])
			}

			pattern := parts[1]
			if len(pattern) == 0 {
				return nil, fmt.Errorf("binary pattern for command '%c' cannot be empty", command)
			}
			for _, p := range pattern {
				if p != '0' && p != '1' {
					return nil, fmt.Errorf("invalid binary pattern for command '%c': %s", command, pattern)
				}
			}

			readEnd := inputPos + count
			if readEnd > endBit {
				readEnd = endBit
			}

			chunk := inputBits[inputPos:readEnd]
			for i, bit := range chunk {
				patternBit := byte(pattern[i%len(pattern)] - '0')
				var resultBit byte
				switch command {
				case 'x':
					resultBit = bit ^ patternBit // XOR
				case 'a':
					resultBit = bit & patternBit // AND
				case 'o':
					resultBit = bit | patternBit // OR
				}
				outputBits.WriteByte(resultBit)
			}
			inputPos = readEnd

		default:
			return nil, fmt.Errorf("unknown command: %c", command)
		}

			if shouldLog && command != 's' {
				bitsAfter := outputBits.Len()
				fmt.Fprintf(os.Stderr, " -> Wrote %d bits to output.\n", bitsAfter-bitsBefore)
			}
		}
		logPrinted = true
	}

	return bitsToBytes(outputBits.Bytes()), nil
}
