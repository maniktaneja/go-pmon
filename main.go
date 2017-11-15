package main

import (
	//"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	tm "github.com/buger/goterm"
	"github.com/shirou/gopsutil/process"
)

const DEFAULT_WAKEUP_TIME = 1

type perfInfo struct {
	pHandle          *process.Process
	Name             string  `json:"process"`
	Cpu              float64 `json:"avg_cpu"`
	CtxSwitchesVol   int64   `json:"ctx_voluntary"`
	CtxSwitchesInvol int64   `json:"ctx_involuntary"`
	Mem              int64   `json:"memory"`
}

var perfInfoMap map[string]*perfInfo

func main() {
	args := os.Args[1:]

	if len(args) < 1 {
		log.Fatalf("No processes to monitor. Usage go-pmon <list of pids>")
	}

	perfInfoMap = make(map[string]*perfInfo)
	for _, p := range args {
		pid, _ := strconv.Atoi(p)
		pHandle, err := process.NewProcess(int32(pid))
		if err != nil {
			log.Printf("Unable to initialize process monitor for pid %s. Error %v", p, err)
			continue
		}
		name, _ := pHandle.Name()

		perfInfoMap[p] = &perfInfo{
			pHandle: pHandle,
			Name:    name,
		}
	}

	done := make(chan bool)
	go runProcessMonitor(done)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	close(done)
	printStats()
}

func runProcessMonitor(done chan bool) {

	var n int64 // number of samples

loop:
	for {
		select {
		case <-time.After(time.Second * DEFAULT_WAKEUP_TIME):
			n++
			for _, pInfo := range perfInfoMap {
				cpu, _ := pInfo.pHandle.CPUPercent()
				ctx, _ := pInfo.pHandle.NumCtxSwitches()
				mem, _ := pInfo.pHandle.MemoryInfo()

				log.Printf(" Sample %v", cpu)
				pInfo.Cpu = approxRollingAverage(pInfo.Cpu, cpu, n)
				pInfo.CtxSwitchesVol = ctx.Voluntary
				pInfo.CtxSwitchesInvol = ctx.Involuntary
				pInfo.Mem = int64(approxRollingAverage(float64(pInfo.Mem), float64(mem.RSS), n))
			}
			printStats()
		case <-done:
			break loop
		}
	}
}

func printStats() {
	// convert to printable stats
	tm.Clear()
	avgs := tm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprintf(avgs, "Name\tCPU\tCTX_voluntary\tCTX_involuntary\tMem\n")
	for _, perfInfo := range perfInfoMap {
		fmt.Fprintf(avgs, "%s\t%v\t%d\t%d\t%s\n", perfInfo.Name, strconv.FormatFloat(float64(perfInfo.Cpu), 'f', 2, 32),
			perfInfo.CtxSwitchesVol, perfInfo.CtxSwitchesInvol, fmt.Sprintf("%v MB ", perfInfo.Mem/(1024*1024)))
	}
	tm.Println(avgs)
	tm.Flush()

}

func approxRollingAverage(avg, new_sample float64, n int64) float64 {

	avg -= avg / float64(n)
	avg += new_sample / float64(n)

	return avg
}
