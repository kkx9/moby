package depbuilder // import "github.com/docker/docker/builder/depbuilder"

import (
	"os"
	"strings"
	"bufio"
	"regexp"
	"strconv"
	"time"

	_"github.com/sirupsen/logrus"
)

type Tracee struct {
	traceLog 	*os.File
	lastTime	int
}

func getTime(timeString string) int {
	timeList := strings.Split(timeString, ":")
	if len(timeList) != 4 {
		return -1
	}
	hour, e1 := strconv.Atoi(timeList[0])
	minute, e2 := strconv.Atoi(timeList[1])
	second, e3 := strconv.Atoi(timeList[2])
	millisecond, e4 := strconv.Atoi(timeList[3])
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return -1
	}
	return ((hour * 60 + minute) * 60 + second) * 10000000  + millisecond

}

func (t *Tracee) UpdateTime() {
	now := time.Now()
	t.lastTime = ((now.Hour() * 60 + now.Minute()) * 60 + now.Second()) * 10000000
}

func (t *Tracee) GetStageId() string {
	traceLogReader := bufio.NewScanner(t.traceLog)
	id := ""
	for {
		if !traceLogReader.Scan() {
			break
		}
		line := strings.TrimSpace(traceLogReader.Text())
		logInfo := strings.Split(line, " ")
		if len(logInfo)  < 10 || logInfo[0] == "TIME"{
			continue
		}
		if getTime(logInfo[0]) < t.lastTime {
			continue
		}
		t.lastTime = getTime(logInfo[0])
		for _,arg := range logInfo[9:] {
			if strings.Contains(arg, "-init") {
				var re = regexp.MustCompile(`(?m)/[a-zA-Z0-9]*-init/`)
				// logrus.Debug(arg)
				for _, match := range re.FindAllString(arg, -1) {
					// logrus.Debug(match)
					if(id == "") {
						id = strings.TrimSuffix(strings.Trim(match,"//"), "-init")
					}
				}
			}
		}
	}
	return id
}

func (t *Tracee) GetDepLayer() []string {
	traceLogReader := bufio.NewScanner(t.traceLog)
	depLayers := make(map[string]bool)
	var re = regexp.MustCompile(`(?m)/vfs/dir/[a-zA-Z0-9]*/`)
	for {
		if !traceLogReader.Scan() {
			break
		}
		line := strings.TrimSpace(traceLogReader.Text())
		logInfo := strings.Split(line, " ")
		if len(logInfo)  < 10 || logInfo[0] == "TIME"{
			continue
		}
		if getTime(logInfo[0]) <= t.lastTime {
			continue
		}
		for _, match := range re.FindAllString(line, -1) {
			depLayers[strings.Split(match, "/")[3]] = true
		}
	}
	j := 0
	keys := make([]string, len(depLayers))
	for k := range depLayers {
		keys[j] = k
		j++
	}
	return keys
}