package daemon

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type DaemonService struct {
	// KeepProcessAlive If it is true, a new goroutine will watch this process.
	// If the process exits without 0 exit-code, it will restart the process.
	KeepProcessAlive bool
	// KeepAliveWatchInterval is the interval to check process state
	KeepAliveWatchInterval time.Duration

	// WaitProcessExit if true, RunProcess blocks until process exits
	WaitProcessExit bool

	// StdOutFile is the output file path
	StdOutFile string
	StdErrFile string
	// LogRotateSize is the size limit of the logs. If the log exceeds the size,
	// the log will be rotated
	LogRotateSize int

	// process info
	ProcessCmd  string
	ProcessArgs []string
	ProcessInfo ProcessInfo
	// IsProcessCmdUnique if rue, it means the process can be found by process cmd nme
	IsProcessCmdUnique bool
	// PIDFIle is the file that records the pid of the process
	PIDFile string

	// cmd
	Command *exec.Cmd

	// watcher fields
	watcherRunning bool
	watcherStopCh  chan struct{}
}

type ProcessInfo struct {
	Cmd, State string
	// ids
	PID, PPID, PGID, SID int
}

func (ds *DaemonService) RunProcess() (err error) {
	var pid int
	var running bool
	// check if process is alive
	pid, running, err = ds.GetProcessInfo()
	if err != nil {
		log.Println(err)
	}

	if !running {
		// start the process
		pid, err = ds.runDaemon()
		if err != nil {
			log.Println(err)
			return
		}
		ds.ProcessInfo.PID = pid
		ds.ProcessInfo, err = FindProcessByPid(pid)
		if err != nil {
			log.Printf("pid in pidfile is %d, but real process is %+v, err %+v \n", pid, ds.ProcessInfo, err)
			return
		}
	}

	err = ioutil.WriteFile(ds.PIDFile, []byte(fmt.Sprintf("%d", pid)), fs.ModePerm)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("process is running at pid %d \n", pid)

	// wait for process quitting
	if ds.WaitProcessExit {
		err = ds.Command.Wait()
		return
	}

	return
}

func (ds *DaemonService) RunWatcher() {
	log.Println("start watching the process")
	ds.watcherRunning = true
	// set default interval to 5s
	if ds.KeepAliveWatchInterval == time.Duration(0) {
		ds.KeepAliveWatchInterval = 5 * time.Second
	}
	ticker := time.NewTicker(ds.KeepAliveWatchInterval)

	var err error

	for {
		select {
		case <-ticker.C:
			var running bool
			if ds.ProcessInfo.PID > 0 {
				var info ProcessInfo
				info, err = FindProcessByPid(ds.ProcessInfo.PID)
				if err != nil {
					log.Println(err, info)
				}
				if info.PPID > 0 {
					ds.ProcessInfo = info
					running = true
					log.Println("process is running ", info)
				}
			} else {
				log.Panicln("unable to watch pid", ds.ProcessInfo.PID)
			}

			if !running {
				log.Println("found process exited")
				err = ds.RunProcess()
				if err != nil {
					log.Println(err)
				} else {
					running = true
				}
			}

		case <-ds.watcherStopCh:
			log.Println("watcher is quitting")
			return
		}
	}
}

func (ds *DaemonService) StopWatcher() {
	ds.watcherRunning = false
	ds.watcherStopCh <- struct{}{}
}

func (ds *DaemonService) runDaemon() (pid int, err error) {
	cmd := exec.Command(ds.ProcessCmd, ds.ProcessArgs...)
	ds.Command = cmd
	cmd.Env = os.Environ()
	cmd.Stdin = nil
	cmd.ExtraFiles = nil

	// setup stdout and stderr
	var stdoutFile, stderrFile io.Writer
	var fileErr error
	if len(ds.StdOutFile) > 0 {
		stdoutFile, fileErr = os.OpenFile(ds.StdOutFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if fileErr != nil {
			log.Println(fileErr)
		} else {
			cmd.Stdout = stdoutFile
		}
	}

	if len(ds.StdErrFile) > 0 {
		stderrFile, fileErr = os.OpenFile(ds.StdErrFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if fileErr != nil {
			log.Println(fileErr)
		} else {
			cmd.Stderr = stderrFile
		}
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Setsid is used to detach the process from the parent (normally a shell)
		//
		// The disowning of a child process is accomplished by executing the system call
		// setpgrp() or setsid(), (both of which have the same functionality) as soon as
		// the child is forked. These calls create a new process session group, make the
		// child process the session leader, and set the process group ID to the process
		// ID of the child. https://bsdmag.org/unix-kernel-system-calls/
		Setsid: true,
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

// GetProcessInfo gets process info. Tt is a read-only operation
func (ds *DaemonService) GetProcessInfo() (pid int, running bool, err error) {
	// check pidfile
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
		// should not trust PIDFile
		// Find process info in /proc/$ID/stat , verify the command is right
		ds.ProcessInfo, err = FindProcessByPid(pid)
		if err != nil {
			log.Printf("pid in pidfile is %d, but real process is %+v, err %+v \n", pid, ds.ProcessInfo, err)
		}

		if ds.ProcessInfo.Cmd != ds.ProcessCmd {
			log.Printf("real process is %+v, want %s \n", ds.ProcessInfo, ds.ProcessCmd)
		}

		if err == nil && ds.ProcessInfo.Cmd == ds.ProcessCmd {
			log.Printf("found process by id %d, process: %+v \n", pid, ds.ProcessInfo)
			running = true
		}
	}

	// cannot find by pidfile; try find by cmd name
	if ds.IsProcessCmdUnique {
		ds.ProcessInfo, err = FindProcessByCmdName(ds.ProcessCmd)
		if err != nil {
			log.Println(err)
			return
		}

		if ds.ProcessInfo.PID > 0 {
			pid = ds.ProcessInfo.PID
			running = true
			log.Printf("found process by id %d, process: %+v \n", pid, ds.ProcessInfo)
		}
	}

	return
}
