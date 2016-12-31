// causalprof inteprets results from causal profiling files
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"cmd/internal/objfile"
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) != 2 {
		usage()
	}
	samples, err := readProfFile(args[0])
	if err != nil {
		fatalln(err.Error())
	}
	// first sample should have the 0 experiment. Keep a reference to it
	nullexp := samples[0]

	// make an index of experiments concerning the same callsite
	index := make(map[uint64][]*sample)
	for _, s := range samples {
		i := index[s.pc]
		i = append(i, s)
		index[s.pc] = i
	}
	// sort each callsite by slowdown
	for _, s := range index {
		sort.Sort(bySpeedup(s))
	}
	// get a symbol table to turn addresses into file:line
	obj, err := objfile.Open(args[1])
	if err != nil {
		fatalln(err.Error())
	}
	pcln, err := obj.PCLineTable()
	if err != nil {
		fatalln(err.Error())
	}
	for pc, i := range index {
		file, line, fn := pcln.PCToLine(pc - 1)
		if fn == nil {
			fmt.Printf("%#x\n", pc)
		} else {
			fmt.Printf("%#x %s:%d\n", pc, file, line)
		}
		fmt.Println(nullexp.nsPerOp)
		for _, s := range i {
			percent := float64(s.nsPerOp-nullexp.nsPerOp) / float64(nullexp.nsPerOp)
			percent *= 100
			fmt.Printf("%3d%%\t%dns\t%+.3g%%\n", s.speedup, s.nsPerOp, percent)
		}
		fmt.Println()
	}

}

type sample struct {
	pc      uint64
	speedup int
	nsPerOp int64
}

type bySpeedup []*sample

func (b bySpeedup) Len() int           { return len(b) }
func (b bySpeedup) Less(i, j int) bool { return b[i].speedup < b[j].speedup }
func (b bySpeedup) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func readProfFile(path string) ([]*sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var samples []*sample
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		s := scan.Text()
		if len(s) < 1 || s[0] == '#' {
			continue
		}
		fields := strings.Fields(s)
		if len(fields) != 3 {
			return nil, fmt.Errorf("corrupt causalprof file, had ", len(fields), "fields; expected 3")
		}
		pc, err := strconv.ParseUint(fields[0], 0, 64)
		if err != nil {
			return nil, err
		}
		speedup, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, err
		}
		nsPerOp, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, err
		}
		samples = append(samples, &sample{
			pc:      pc,
			speedup: speedup,
			nsPerOp: nsPerOp,
		})
	}
	return samples, scan.Err()
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: causalprof file program")
	os.Exit(1)
}

func fatalln(err string) {
	fmt.Fprintln(os.Stderr, "causalprof:", err)
	os.Exit(1)
}
