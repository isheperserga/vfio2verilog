package parser

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

type ByteVals map[uint][]uint8

type WordResp struct {
	BaseAddr uint32
	ByteVals ByteVals
	WordVals []uint32
}

type DebugStuff struct {
	ByteReadCnt int
	WordReadCnt int
	BytePosMap  map[uint32]int
	WordPosMap  map[uint32]int
	ByteUniqCnt map[uint32]int
	WordUniqCnt map[uint32]int
}

type LogOp struct {
	Idx    int
	Addr   uint32
	Size   uint32
	Val    uint32
	Region uint32
}

func (wr *WordResp) HasManyResps() bool {
	if len(wr.UniqWordVals()) > 1 {
		return true
	}

	for off := uint(0); off <= 3; off++ {
		if len(wr.UniqByteVals(off)) > 1 {
			return true
		}
	}

	return false
}

func (wr *WordResp) UniqByteVals(off uint) []uint8 {
	vals, exists := wr.ByteVals[off]
	if !exists {
		return nil
	}

	uniqVals := make([]uint8, 0)
	seenStuff := make(map[uint8]struct{})

	for _, v := range vals {
		if _, ok := seenStuff[v]; !ok {
			seenStuff[v] = struct{}{}
			uniqVals = append(uniqVals, v)
		}
	}

	return uniqVals
}

func (wr *WordResp) UniqWordVals() []uint32 {
	uniqVals := make([]uint32, 0)
	seenStuff := make(map[uint32]struct{})

	for _, v := range wr.WordVals {
		if _, ok := seenStuff[v]; !ok {
			seenStuff[v] = struct{}{}
			uniqVals = append(uniqVals, v)
		}
	}

	return uniqVals
}

func (wr *WordResp) GetByteValsForOff(off uint) []uint8 {
	if vals, exists := wr.ByteVals[off]; exists && len(vals) > 0 {
		return vals
	}
	return []uint8{}
}

func (wr *WordResp) AddByteVal(off uint, val uint8) {
	if _, exists := wr.ByteVals[off]; !exists {
		wr.ByteVals[off] = []uint8{}
	}
	wr.ByteVals[off] = append(wr.ByteVals[off], val)
}

func (wr *WordResp) AddWordVal(val uint32) {
	wr.WordVals = append(wr.WordVals, val)
}

func ParseVFIOLogOps(logfile string) ([]LogOp, error) {
	file, err := os.Open(logfile)
	if err != nil {
		return nil, fmt.Errorf("cant open: %w", err)
	}
	defer file.Close()

	readPat := regexp.MustCompile(`vfio_region_read\s*\(.*?:region([0-9]+)\+0x([0-9a-fA-F]+),\s*(\d+)\s*\)\s*=\s*0x([0-9a-fA-F]+)`)
	writePat := regexp.MustCompile(`vfio_region_write\s*\(\s*.*?\+0x([0-9a-fA-F]+),\s*0x([0-9a-fA-F]+),\s*(\d+)\s*\)`)

	ops := []LogOp{}
	scanner := bufio.NewScanner(file)
	bigBuf := make([]byte, 1024*1024)
	scanner.Buffer(bigBuf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		txt := scanner.Text()
		if matches := readPat.FindStringSubmatch(txt); len(matches) >= 5 {
			regionStr, addrStr, sizeStr, valStr := matches[1], matches[2], matches[3], matches[4]
			region, _ := strconv.ParseUint(regionStr, 10, 32)
			addrU64, _ := strconv.ParseUint(addrStr, 16, 32)
			sizeU64, _ := strconv.ParseUint(sizeStr, 10, 32)
			valU64, _ := strconv.ParseUint(valStr, 16, 64)
			ops = append(ops, LogOp{
				Idx:    lineNum,
				Addr:   uint32(addrU64),
				Size:   uint32(sizeU64),
				Val:    uint32(valU64),
				Region: uint32(region),
			})
		}
		if matches := writePat.FindStringSubmatch(txt); len(matches) >= 4 {
			// TODO: :')
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read fail: %w", err)
	}
	return ops, nil
}
