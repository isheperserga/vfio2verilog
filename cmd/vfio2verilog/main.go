package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/isheperserga/vfio2verilog/internal/generator"
	"github.com/isheperserga/vfio2verilog/internal/parser"
)

func main() {
	log.SetPrefix("vfio2v: ")

	inFile := flag.String("input", "", "vfio log file")
	outFile := flag.String("output", "gen_ctrl.sv", "sv file")
	modName := flag.String("module", "pcileech_impl_bar_controller", "module name")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [options] [logfile]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *inFile == "" && flag.NArg() > 0 {
		*inFile = flag.Arg(0)
	}

	if *inFile == "" {
		log.Fatal("no input file! use -in or give as arg")
	}

	if _, err := os.Stat(*inFile); os.IsNotExist(err) {
		log.Fatalf("cant find file: %s", *inFile)
	}

	outDir := filepath.Dir(*outFile)
	if outDir != "." && outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			log.Fatalf("cant make dir: %v", err)
		}
	}

	fmt.Printf("parsing log: %s\n", *inFile)
	ops, err := parser.ParseVFIOLogOps(*inFile)
	if err != nil {
		log.Fatalf("parse fail: %v", err)
	}

	barOps := map[uint32][]parser.LogOp{}
	for _, op := range ops {
		barOps[op.Region] = append(barOps[op.Region], op)
	}

	fmt.Printf("detected %d bars\n", len(barOps))
	for region, ops := range barOps {
		fmt.Printf("found %d ops on bar%d\n", len(ops), region)
	}

	var allVerilog strings.Builder
	for region, ops := range barOps {
		thisModName := fmt.Sprintf("%s%d", *modName, region)
		fmt.Printf("making verilog for bar%d\n", region)
		verilogCode, err := generator.GenerateVerilogFromOps(ops, thisModName)
		if err != nil {
			log.Fatalf("gen fail for bar%d: %v", region, err)
		}
		allVerilog.WriteString(verilogCode)
		allVerilog.WriteString("\n\n")
	}
	if err := os.WriteFile(*outFile, []byte(allVerilog.String()), 0644); err != nil {
		log.Fatalf("save fail: %v", err)
	}
	fmt.Printf("saved all modules to %s\n", *outFile)
}
