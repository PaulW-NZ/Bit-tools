package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

func printUsage() {
	fmt.Println("Usage: crc [options] <file>")
	fmt.Println("Options:")
	flag.VisitAll(func(f *flag.Flag) {
		format := "  -%-10s %s"
		value := f.DefValue
		switch f.Name {
		case "poly", "init", "xorout":
			// Parse the default value and format as hex
			num, err := strconv.ParseUint(f.DefValue, 10, 64)
			if err == nil {
				value = fmt.Sprintf("0x%x", num)
			}
		}
		fmt.Printf(format, f.Name, f.Usage)
		fmt.Printf(" (default %s)\n", value)
	})
	fmt.Println("\nCommon Standards:")
	fmt.Println("  CRC-32 (default): -width=32 -poly=0x4c11db7 -init=0xffffffff -xorout=0xffffffff")
	fmt.Println("  CRC-16/MODBUS:    -width=16 -poly=0x8005  -init=0xffff     -xorout=0x0")
	fmt.Println("  CRC-8/DARC:       -width=8  -poly=0x39    -init=0x0        -xorout=0x0")
}

func main() {
	// --- Command-Line Flags ---
	poly := flag.Uint("poly", 0x04C11DB7, "generator polynomial (normal form)")
	initVal := flag.Uint64("init", 0xFFFFFFFF, "initial value")
	xorOut := flag.Uint64("xorout", 0xFFFFFFFF, "final XOR value")
	width := flag.Int("width", 32, "CRC width in bits (8, 16, 32)")

	flag.Usage = printUsage
	flag.Parse()

	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	filePath := flag.Arg(0)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %s", err)
	}

	switch *width {
	case 32:
		finalCrc := calculateCRC32(data, uint32(*poly), uint32(*initVal), uint32(*xorOut))
		fmt.Printf("CRC-32 for %s: 0x%08x\n", filePath, finalCrc)
	case 16:
		finalCrc := calculateCRC16(data, uint16(*poly), uint16(*initVal), uint16(*xorOut))
		fmt.Printf("CRC-16 for %s: 0x%04x\n", filePath, finalCrc)
	case 8:
		finalCrc := calculateCRC8(data, uint8(*poly), uint8(*initVal), uint8(*xorOut))
		fmt.Printf("CRC-8 for %s: 0x%02x\n", filePath, finalCrc)
	default:
		log.Fatalf("Unsupported CRC width: %d", *width)
	}
}

// --- CRC-32 Implementation ---
func calculateCRC32(data []byte, poly, initVal, xorOut uint32) uint32 {
	reflectedPoly := reflect32(poly)
	table := crc32.MakeTable(reflectedPoly)

	crc := initVal
	for _, b := range data {
		crc = table[byte(crc)^b] ^ (crc >> 8)
	}
	return crc ^ xorOut
}

func reflect32(data uint32) uint32 {
	var r uint32
	for i := 0; i < 32; i++ {
		if (data&(1<<i)) != 0 {
			r |= 1 << (31 - i)
		}
	}
	return r
}

// --- CRC-16 Implementation ---
func calculateCRC16(data []byte, poly, initVal, xorOut uint16) uint16 {
	reflectedPoly := reflect16(poly)
	table := makeTable16(reflectedPoly)

	crc := initVal
	for _, b := range data {
		crc = table[byte(crc)^b] ^ (crc >> 8)
	}
	return crc ^ xorOut
}

func makeTable16(poly uint16) *[256]uint16 {
	var table [256]uint16
	for i := 0; i < 256; i++ {
		crc := uint16(i)
		for j := 0; j < 8; j++ {
			if (crc&1) == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		table[i] = crc
	}
	return &table
}

func reflect16(data uint16) uint16 {
	var r uint16
	for i := 0; i < 16; i++ {
		if (data&(1<<i)) != 0 {
			r |= 1 << (15 - i)
		}
	}
	return r
}

// --- CRC-8 Implementation ---
func calculateCRC8(data []byte, poly, initVal, xorOut uint8) uint8 {
	reflectedPoly := reflect8(poly)
	table := makeTable8(reflectedPoly)

	crc := initVal
	for _, b := range data {
		crc = table[crc^b]
	}
	return crc ^ xorOut
}

func makeTable8(poly uint8) *[256]uint8 {
	var table [256]uint8
	for i := 0; i < 256; i++ {
		crc := uint8(i)
		for j := 0; j < 8; j++ {
			if (crc&1) == 1 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		table[i] = crc
	}
	return &table
}

func reflect8(data uint8) uint8 {
	var r uint8
	for i := 0; i < 8; i++ {
		if (data&(1<<i)) != 0 {
			r |= 1 << (7 - i)
		}
	}
	return r
}