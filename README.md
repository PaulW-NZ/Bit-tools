# Bit-tools 

A collection of command-line utilities for low-level, bit-by-bit data manipulation.

## Tools Included

- **`bit-editor`**: A tool for applying a chain of transformations (take, skip, invert, etc.) to a binary file.
- **`interleaver`**: A tool for re-ordering, multiplexing, and de-multiplexing data streams at the bit, byte, or word level.
- **`lfsr`**: A tool for generating, encrypting/decrypting, and scrambling/descrambling data using Linear Feedback Shift Registers.
- **`crc`**: A flexible tool for calculating Cyclic Redundancy Checks (CRCs) of various bit widths.
- **`hamming`**: A tool for encoding and decoding data with error-correcting Hamming codes.

## Building

To build the tools from source, you need to have [Go](https://golang.org/) installed.

Clone the repository and run the following command in the project directory to build all executables:

```bash
go build -o bit-editor bit-editor.go && go build -o interleaver interleaver.go && go build -o lfsr lfsr.go && go build -o crc crc.go && go build -o hamming hamming.go
```

---

## `bit-editor`

A powerful and flexible command-line tool for performing low-level, bit-by-bit manipulation on binary files.

### Features

- **Stream-Based Processing**: Applies a repeating sequence of commands to a file from a start to an end bit.
- **Rich Command Set**: Includes commands for taking, skipping, inserting, inverting, reversing bits, byte-swapping, and performing logical (XOR, AND, OR) operations.
- **Hybrid Block Operations**: Group commands into a chain (e.g., `[vn]8`) to apply multiple transformations to a single chunk of data efficiently.
- **Precise Range Selection**: Use `--start` and `--end` flags to limit operations to a specific bit range within a file.
- **Debugging & Simulation**: A `--verbose` mode to see step-by-step operations, a `--verbose-once` mode for cleaner logs on large files, and a `--dry-run` mode to validate commands without writing any data.
- **Unix-Friendly**: Supports piping from `stdin` and to `stdout`, allowing it to be easily integrated into command-line workflows.

### Usage (`bit-editor`)

The tool is run with an edit string (`-e`) and optional flags for input, output, and range selection.

```bash
./bit-editor -e "<commands>" [flags...]
```

#### Flags

| Flag             | Description                                                                  |
| ---------------- | ---------------------------------------------------------------------------- |
| `-e <string>`      | **(Required)** The repeating string of edit commands.                        |
| `-i <file>`        | Input file path. Defaults to standard input.                                 |
| `-o <file>`        | Output file path. Defaults to standard output.                               |
| `--start <int>`    | The bit position to start editing from (inclusive). Defaults to 0.           |
| `--end <int>`      | The bit position to stop editing at (exclusive). Defaults to the end of data. |
| `--verbose`        | Enable verbose logging for every loop of the command sequence.               |
| `--verbose-once`   | Enable verbose logging for the first command sequence loop only.             |
| `--dry-run`        | Simulate operations and report what the output size would be.                |
| `--help`           | Show the detailed help message.                                              |


#### Command Language

- `t<number>`: **Take** `<number>` bits from the input stream.
- `s<number>`: **Skip** `<number>` bits from the input stream.
- `i<binary>`: **Insert** a literal `<binary>` string into the output.
- `n<number>`: **Invert** (flip) the next `<number>` bits from the input stream.

#### Re-ordering Operations
- `v<number>`: **Reverse** the order of BITS within the next `<number>`-bit word.
- `b<number>`: **Reverse** the order of BYTES within the next `<number>`-bit word (for endian swapping).

#### Logical Operations
- `x<N>:<P>`: **XOR** the next `<N>` bits with the repeating binary pattern `<P>`.
- `a<N>:<P>`: **AND** the next `<N>` bits with the repeating binary pattern `<P>`.
- `o<N>:<P>`: **OR** the next `<N>` bits with the repeating binary pattern `<P>`.

#### Block Operations
- `[<chain>]<N>`: Processes the next `<N>` bits as a single block, applying the `<chain>` of commands to it. (Allowed in chain: `n, v, b, x, a, o`).


### Examples (`bit-editor`)

**1. Change the endianness of a file containing 32-bit little-endian words:**
```bash
./bit-editor -e "b32" -i in.dat -o out.dat
```

**2. Reverse and Invert each byte of a file using a block operation:**
```bash
./bit-editor -e "[vn]8" -i in.dat -o out.dat
```

**3. Apply a simple XOR cipher to a file with verbose logging for the first loop:**
```bash
./bit-editor -e "x8:10110101" --verbose-once -i secret.dat -o encoded.dat
```

---

## `interleaver`

A tool for re-ordering, multiplexing (muxing), and de-multiplexing (de-muxing) data streams at the bit, byte, or word level.

### Features

- **Three Operating Modes**: Permute elements in-place, mux multiple files into one, or de-mux one file into many.
- **Arbitrary Element Size**: Operates on elements of any bit size in Permute mode, and any byte-aligned size in Mux/De-mux modes.
- **Powerful Permutation**: Supports any valid permutation for re-ordering elements.
- **Inverse Operation**: Can automatically calculate and apply the inverse of a permutation to restore the original order.

### Usage & Modes (`interleaver`)

The tool's mode is determined by the flags you provide.

#### 1. Permute Mode
Re-orders elements within a single file. **Triggered by the `-p` flag.**

- **Syntax:** `./interleaver -p "<pattern>" -s <size> [flags...]`
- **Example:** Swap every pair of bytes.
    ```bash
    ./interleaver -p "1,0" -s 8 -i in.dat -o out.dat
    ```

#### 2. Interleave (Mux) Mode
Combines multiple files into one. **Triggered by providing multiple input files as arguments.**

- **Syntax:** `./interleaver -s <size> -o <out.dat> <in1.dat> <in2.dat> ...`
- **Example:** Interleave three files byte by byte.
    ```bash
    # f1="AAA", f2="BBB", f3="CCC" -> combined.dat="ABCABCABC"
    ./interleaver -s 8 -o combined.dat f1.dat f2.dat f3.dat
    ```

#### 3. De-interleave (De-mux) Mode
Splits one file into many. **Triggered by the `--split` flag.**

- **Syntax:** `./interleaver -s <size> --split <n> -i <in.dat>`
- **Example:** Split a file into 3 streams.
    ```bash
    # combined.dat="ABCABCABC" -> combined_0.dat="AAA", combined_1.dat="BBB", ...
    ./interleaver -s 8 --split 3 -i combined.dat
    ```

---

## `lfsr`

A tool for generating, encrypting/decrypting, and scrambling/descrambling data using Linear Feedback Shift Registers (LFSRs).

### Core Concepts

- **Polynomial (`-p`):** Defines the LFSR's feedback logic as a comma-separated list of tap positions (e.g., `"16,14,13,11"`). The highest tap defines the degree (size) of the LFSR.
- **Initial Fill/Seed (`-s`):** The starting state of the register, provided as a binary string (e.g., `"1001000010010011"`). Its length must match the polynomial's degree.

### Usage & Modes (`lfsr`)

The tool's mode is determined by the `--mode` flag.

#### 1. Generate Sequence (`--mode=gen`)
Generates a raw LFSR output sequence.

- **Syntax:** `./lfsr --mode=gen -p "<poly>" -s "<seed>" -n <num_bits> [-o out.dat]`
- **Example:** Generate 8 bits from a 4-bit LFSR.
    ```bash
    ./lfsr --mode=gen -p "4,1" -s "1000" -n 8 | xxd -b
    # Expected output: 00000000: 00011110
    ```

#### 2. Stream Cipher (`--mode=cipher`)
Applies the LFSR sequence as a simple XOR stream cipher to data. The LFSR runs independently of the data stream. The process is identical for encrypting and decrypting.

- **Syntax:** `./lfsr --mode=cipher -p "<poly>" -s "<seed>" [-i in.dat] [-o out.dat]`
- **Example:** Encrypt and decrypt a file.
    ```bash
    echo -n "Hello, stream cipher!" > plain.txt
    ./lfsr --mode=cipher -p "16,14,13,11" -s "1001000010010011" -i plain.txt -o cipher.dat
    ./lfsr --mode=cipher -p "16,14,13,11" -s "1001000010010011" -i cipher.dat -o decrypted.txt
    diff plain.txt decrypted.txt # Should produce no output
    ```

#### 3. Feed-Through Scrambler (`--mode=scramble`)
Scrambles a data stream using a self-synchronizing LFSR. The LFSR's state is influenced by the input data.

- **Syntax:** `./lfsr --mode=scramble -p "<poly>" [-i in.dat] [-o out.dat]`
- **Example:** Scramble a file.
    ```bash
    echo -n "Hello, scrambler!" > plain_scramble.txt
    ./lfsr --mode=scramble -p "16,14,13,11" -i plain_scramble.txt -o scrambled.dat
    ```

#### 4. Feed-Through Descrambler (`--mode=descramble`)
Descrambles a data stream that was previously scrambled using the same polynomial. This mode is also self-synchronizing.

- **Syntax:** `./lfsr --mode=descramble -p "<poly>" [-i in.dat] [-o out.dat]`
- **Example:** Descramble a file.
    ```bash
    ./lfsr --mode=descramble -p "16,14,13,11" -i scrambled.dat -o descrambled.txt
    diff plain_scramble.txt descrambled.txt # Should produce no output
    ```

---

## `crc`

A flexible tool for calculating Cyclic Redundancy Checks (CRCs).

### Features

- **Multiple Widths**: Supports 8, 16, and 32-bit CRC calculations.
- **Custom Parameters**: Allows specifying a custom generator polynomial, initial value, and final XOR value.
- **Algorithm Handling**: Automatically handles the underlying details of reflected, little-endian CRC calculation.
- **Informative Help**: Includes examples of common CRC standards (CRC-32, MODBUS, DARC) in its help message.

### Usage (`crc`)

```bash
./crc [flags...] <file>
```

#### Flags

| Flag          | Description                                  |
| ------------- | -------------------------------------------- |
| `-width <int>`  | CRC width in bits (8, 16, 32). Defaults to 32. |
| `-poly <hex>`   | Generator polynomial in normal form.         |
| `-init <hex>`   | Initial value of the CRC register.           |
| `-xorout <hex>` | The value to XOR with the final CRC.         |

### Examples (`crc`)

**1. Calculate the default CRC-32 for a file:**
```bash
./crc README.md
```

**2. Calculate the CRC-16/MODBUS checksum for a file:**
```bash
./crc -width=16 -poly=0x8005 -init=0xffff -xorout=0 some_file.dat
```

---

## `hamming`

A tool for encoding and decoding data using Hamming codes, capable of automatically correcting single-bit errors.

### Features

- **Generic Implementation**: Supports any standard Hamming code `(2^m-1, 2^m-1-m)` via the `-m` flag (e.g., (7,4), (15,11), (31,26)).
- **Extended Code Support**: Can use extended Hamming codes (e.g., (8,4)) to detect 2-bit errors.
- **Error Correction**: Automatically corrects single-bit errors in each block of data during decoding.
- **Verbose Reporting**: An optional `-v` flag reports when and where corrections occurred.
- **Uncorrectable Error Warnings**: Detects and warns about uncorrectable 2-bit errors when using extended codes.

### Usage (`hamming`)

The tool is run in either encode or decode mode.

```bash
# Encode
./hamming -encode [-m <m>] [-extended] -i <infile> -o <outfile>

# Decode
./hamming -decode [-m <m>] [-extended] [-v] -i <infile> -o <outfile>
```

#### Flags

| Flag        | Description                                                                                             |
| ----------- | ------------------------------------------------------------------------------------------------------- |
| `-encode`   | Run in encode mode.                                                                                     |
| `-decode`   | Run in decode mode.                                                                                     |
| `-i <file>`   | Input file path. Defaults to standard input.                                                            |
| `-o <file>`   | Output file path. Defaults to standard output.                                                          |
| `-m <int>`    | Sets the `m` parameter for the code, defining `(2^m-1, 2^m-1-m)`. Defaults to 3 for Hamming(7,4).        |
| `-extended` | Use the extended version of the selected Hamming code (e.g., (8,4) if `-m=3`).                            |
| `-v`        | Verbose mode (decode only). Prints a message to stderr each time a 1-bit error is corrected.              |

### Examples (`hamming`)

**1. Protect a file with standard Hamming(7,4) and then decode it:**
```bash
# Encode the file
./hamming -encode -i plain.txt -o encoded.ham

# Corrupt a single bit in the encoded file (for demonstration)
# (Assuming a tool or script to flip a bit at a certain position)

# Decode the file; errors will be silently corrected
./hamming -decode -i encoded.ham -o decoded.txt
```

**2. Use extended Hamming(8,4) and see verbose output for a corrected error:**
```bash
# Encode with -m=3 and -extended
./hamming -encode -m=3 -extended -i plain.txt -o encoded_ext.ham

# Corrupt a bit in encoded_ext.ham...

# Decode with verbose flag to see the correction report
./hamming -decode -m=3 -extended -v -i encoded_ext.ham -o decoded_ext.txt
# Stderr will show: "Corrected 1-bit error in block X at position Y"
```

**3. Use a larger Hamming(15,11) code:**
```bash
# Encode with -m=4
./hamming -encode -m=4 -i large_file.dat -o encoded_15_11.ham

# Decode
./hamming -decode -m=4 -i encoded_15_11.ham -o decoded_large_file.dat
```

---

## Credits

This suite of tools was developed with the invaluable assistance of **Gemini**, a large language model by Google, demonstrating its capabilities in software engineering and interactive development.
