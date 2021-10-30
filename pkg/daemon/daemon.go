package daemon

import (
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

type DaemonService struct {
	// KeepProcessAlive If it is true, a new goroutine will watch this process.
	// If the process exits without 0 exit-code, it will restart the process.
	KeepProcessAlive bool

	// StdOutFile is the output file path
	StdOutFile string
	StdErrFIle string
	// LogRotateSize is the size limit of the logs. If the log exceeds the size,
	// the log will be rotated
	LogRotateSize int

	// process info
	ProcessCmd  string
	ProcessArgs []string
	// IsProcessCmdUnique if rue, it means the process can be found by process cmd nme
	IsProcessCmdUnique bool
	// PIDFIle is the file that records the pid of the process
	PIDFile string
}

type ProcessIDInfo struct {
	PID, PPID, PGID, SID int
}

func Run() {
	// check if process is alive
	// check pid file
	// check process name
}

//
func (ds *DaemonService) GetPID() (pid int, running bool, err error) {
	if len(ds.PIDFile) == 0 {
		err = fmt.Errorf("pidfile is empty")
		return
	}

	bs, err := ioutil.ReadFile(ds.PIDFile)
	if err != nil {
		log.Println("read pidfile failed: ", ds.PIDFile, err)
	}

	pid, err = strconv.Atoi(strings.TrimSpace(string(bs)))
	if err != nil {
		log.Println("content in pidfile is not number", ds.PIDFile, string(bs), err)
	}

	if pid > 0 {
		// TODO: should trust PIDFile???
		// Find process info in /proc/$ID/stat
		/*
			Example:
			cat /proc/1716828/stat
			1716828 (caddy) S 1 1716828 1716828 0 -1 1077936384 518499 0 13489 0 88881 89117 0 0 20 0 11 0 1155533824 122359808 6246 18446744073709551615 1 1 0 0 0 0 0 0 2143420159 0 0 0 17 0 0 0 339 0 0 0 0 0 0 0 0 0 0
		*/
		running = true
		return
	}

	// cannot find by pidfile; try find by cmd name
	if ds.IsProcessCmdUnique {

	}

	return
}
