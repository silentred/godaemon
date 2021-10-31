package main

import (
	"log"

	"github.com/silentred/godaemon/pkg/daemon"
)

func main() {
	dm := &daemon.DaemonService{
		KeepProcessAlive: true,

		StdOutFile: "/tmp/out.log",
		StdErrFile: "/tmp/error.log",

		ProcessCmd:  "sleep",
		ProcessArgs: []string{"180"},
		PIDFile:     "/tmp/test.pid",
	}

	err := dm.RunProcess()
	if err != nil {
		log.Println(err)
		return
	}

	dm.RunWatcher()
}
