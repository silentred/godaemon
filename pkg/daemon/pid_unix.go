package daemon

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
)

// FindPIDByCmdName finds process by name
func FindProcessByCmdName(cmd string) (info ProcessInfo, err error) {
	// ps xao pid,ppid,sid,command | grep $NAME | grep -v grep
	var out []byte
	var args = []string{"-c", fmt.Sprintf("ps xao pid,ppid,pgid,sid,state,comma`nd | grep %s | grep -v grep", cmd)}
	out, err = exec.Command("sh", args...).CombinedOutput()
	if err != nil {
		return
	}

	err = handleFieldsOfEachLine(out, func(cols []string) error {
		var err error
		if len(cols) >= 6 {
			var pid, ppid, pgid, sid, state, command string
			var errStr error
			pid = cols[0]
			ppid = cols[1]
			pgid = cols[2]
			sid = cols[3]
			state = cols[4]
			command = cols[5]

			info.PID, errStr = strconv.Atoi(pid)
			if errStr != nil {
				err = fmt.Errorf("pid is %s, err %+v", pid, errStr)
				return err
			}

			info.PPID, errStr = strconv.Atoi(ppid)
			if errStr != nil {
				err = fmt.Errorf("ppid is %s, err %+v", ppid, errStr)
				return err
			}

			info.PGID, errStr = strconv.Atoi(pgid)
			if errStr != nil {
				err = fmt.Errorf("pgid is %s, err %+v", pgid, errStr)
				return err
			}

			info.SID, errStr = strconv.Atoi(sid)
			if errStr != nil {
				err = fmt.Errorf("sid is %s, err %+v", sid, errStr)
				return err
			}

			info.Cmd = command
			info.State = state
		}
		return err
	})

	if err != nil {
		return
	}

	if info.PID == 0 {
		err = fmt.Errorf("cannot find process by cmd %s, output: %s", cmd, string(out))
	}

	return
}

/*
FindProcessByPid finds process info in /proc/$ID/stat.

Example:
cat /proc/1716828/stat
1716828 (caddy) S 1 1716828 1716828 0 -1 1077936384 518499 0 13489 0 88881 89117 0 0 20 0 11 0 1155533824 122359808 6246 18446744073709551615 1 1 0 0 0 0 0 0 2143420159 0 0 0 17 0 0 0 339 0 0 0 0 0 0 0 0 0 0
*/
func FindProcessByPid(pid int) (info ProcessInfo, err error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	dataBytes, err := ioutil.ReadFile(statPath)
	if err != nil {
		return
	}

	// First, parse out the image name
	data := string(dataBytes)
	binStart := strings.IndexRune(data, '(') + 1
	binEnd := strings.IndexRune(data[binStart:], ')')
	info.Cmd = data[binStart : binStart+binEnd]

	// Move past the image name and start parsing the rest
	data = data[binStart+binEnd+2:]
	_, err = fmt.Sscanf(data,
		"%s %d %d %d",
		&info.State,
		&info.PPID,
		&info.PGID,
		&info.SID)

	info.PID = pid

	return
}

func handleFieldsOfEachLine(out []byte, lineFn func(cols []string) error) (err error) {
	var line string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	// The split function defaults to ScanLines.
	for scanner.Scan() {
		line = scanner.Text()
		stripedLine := strings.TrimSpace(line)
		if len(stripedLine) > 0 {
			cols := strings.Fields(stripedLine)
			err = lineFn(cols)
			if err != nil {
				return err
			}
		}
	}

	var errRead = scanner.Err()
	if errRead != nil && errRead != io.EOF {
		err = errRead
		return
	}

	return
}
