package depbuilder // import "github.com/docker/docker/builder/depbuilder"

import (
	"os"
	"strings"
	"bufio"
	"regexp"
	_ "strconv"
	"time"
	"encoding/json"

	_"github.com/sirupsen/logrus"
)

type Tracee struct {
	traceLog 			*os.File
	lastTime			int64
	layerDigest			string
	layerCount			int
	layerDict			map[int]string
	fileUpdateRecord	map[string]int
	traceRecord			[]map[string]interface{}
}

const dockerStorgePath = "var/lib/docker/vfs/dir/"

var dockerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/vfs/dir/`)

var initLayerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/vfs/dir/[0-9a-zA-Z]+-init/`)

var normalLayerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/vfs/dir/[0-9a-zA-Z]+/`)

var totalLayerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/vfs/dir/[0-9a-zA-Z]+(-init)/`)

const normalLayerReg = `(?m)^/var/lib/docker/vfs/dir/`



// func getTime(timeString string) int {
// 	timeList := strings.Split(timeString, ":")
// 	if len(timeList) != 4 {
// 		return -1
// 	}
// 	hour, e1 := strconv.Atoi(timeList[0])
// 	minute, e2 := strconv.Atoi(timeList[1])
// 	second, e3 := strconv.Atoi(timeList[2])
// 	millisecond, e4 := strconv.Atoi(timeList[3])
// 	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
// 		return -1
// 	}
// 	return ((hour * 60 + minute) * 60 + second) * 10000000  + millisecond

// }

func (t *Tracee) UpdateTime() {
	t.lastTime = time.Now().UnixNano()
}

// func (t *Tracee) GetStageId() string {
// 	traceLogReader := bufio.NewScanner(t.traceLog)
// 	id := ""
// 	for {
// 		if !traceLogReader.Scan() {
// 			break
// 		}
// 		line := strings.TrimSpace(traceLogReader.Text())
// 		logInfo := strings.Split(line, " ")
// 		if len(logInfo)  < 10 || logInfo[0] == "TIME"{
// 			continue
// 		}
// 		if getTime(logInfo[0]) < t.lastTime {
// 			continue
// 		}
// 		t.lastTime = getTime(logInfo[0])
// 		for _,arg := range logInfo[9:] {
// 			if strings.Contains(arg, "-init") {
// 				var re = regexp.MustCompile(`(?m)/[a-zA-Z0-9]*-init/`)
// 				// logrus.Debug(arg)
// 				for _, match := range re.FindAllString(arg, -1) {
// 					// logrus.Debug(match)
// 					if(id == "") {
// 						id = strings.TrimSuffix(strings.Trim(match,"//"), "-init")
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return id
// }

// func (t *Tracee) GetDepLayer() []string {
// 	traceLogReader := bufio.NewScanner(t.traceLog)
// 	depLayers := make(map[string]bool)
// 	var re = regexp.MustCompile(`(?m)/vfs/dir/[a-zA-Z0-9]*/`)
// 	for {
// 		if !traceLogReader.Scan() {
// 			break
// 		}
// 		line := strings.TrimSpace(traceLogReader.Text())
// 		logInfo := strings.Split(line, " ")
// 		if len(logInfo)  < 10 || logInfo[0] == "TIME"{
// 			continue
// 		}
// 		if getTime(logInfo[0]) <= t.lastTime {
// 			continue
// 		}
// 		for _, match := range re.FindAllString(line, -1) {
// 			depLayers[strings.Split(match, "/")[3]] = true
// 		}
// 	}
// 	j := 0
// 	keys := make([]string, len(depLayers))
// 	for k := range depLayers {
// 		keys[j] = k
// 		j++
// 	}
// 	return keys
// }

func (t *Tracee) GetStageId() string {
	for _, trace := range t.traceRecord {
		argsList := trace["args"].([]interface{})
		mkdirPath := getPath(argsList)
		if strings.Contains(mkdirPath, "-init") && strings.HasPrefix(mkdirPath, dockerStorgePath) {
			var re = regexp.MustCompile(`(?m)/[a-zA-Z0-9]*-init/`)
			for _, match := range re.FindAllString(mkdirPath, -1) {
				return strings.TrimSuffix(strings.Trim(match,"//"), "-init")
			}
		}
	}
	return ""
}

func (t *Tracee) GetDepLayer() []int {
	rL, _ := t.MatchTrace()
	return rL
	// for _, trace := range t.traceRecord {
	// 	argsList := trace["args"].([]interface{})
	// 	callPath := getPath(argsList)
	// 	for _, match := range normalLayerPrefix.FindAllString(callPath, -1) {
	// 		filePath = strings.Trim(callPath, match)
	// 	}
		
	// }
}

func unmarshalTrace(line string) (map[string]interface{}, error) {
	var trace map[string]interface{}
	err := json.Unmarshal([]byte(line), &trace)
	return trace, err
}

func getPath(argsList []interface{}) string {
	for _, arg := range argsList {
		argMap := arg.(map[string]interface{})
		if argMap["name"].(string) == "pathname" {
			return argMap["value"].(string)
		}
	}
	return ""
}

func getMode(argsList []interface{}) string {
	for _, arg := range argsList {
		argMap := arg.(map[string]interface{})
		if argMap["name"].(string) == "flags" {
			return argMap["value"].(string)
		}
	}
	return ""
}

func (t *Tracee) MatchTrace() ([]int, error) {
	var openList, mkdirList []map[string]interface{}
	t.traceRecord = t.traceRecord[:0]
	// openList := make(map[string]interface{},)
	// mkdirList := []map[string]interface{}{}
	traceLogReader := bufio.NewScanner(t.traceLog)
	for {
		if !traceLogReader.Scan() {
			break
		}
		line := strings.TrimSpace(traceLogReader.Text())
		trace, err := unmarshalTrace(line)

		if err != nil {
			return nil, err
		}

		if trace["timestamp"].(int64) <= t.lastTime{
			continue
		}

		t.traceRecord = append(t.traceRecord, trace)

		if trace["eventName"] == "mkdir" || trace["eventName"] == "mkdirat" {
			mkdirList = append(mkdirList, trace)
		} else {
			openList = append(openList, trace)
		}
	}

	fileMap := make(map[string]bool)

	for _, trace := range mkdirList {
		argsList := trace["args"].([]interface{})
		mkdirPath := getPath(argsList)
		if strings.HasPrefix(mkdirPath, dockerStorgePath) {
			//check layer
			re := regexp.MustCompile(normalLayerReg + t.layerDigest + `/`)
			if regMatch := re.FindAllString(mkdirPath, -1); len(regMatch) > 0 {
				pathSuffix := strings.TrimPrefix(mkdirPath, dockerStorgePath + t.layerDigest + `/`)
				if len(pathSuffix) > 0 {
					if _, inMap := fileMap[pathSuffix]; inMap {
						t.fileUpdateRecord[pathSuffix] = t.layerCount
						delete(fileMap, pathSuffix)
					}
				}		
			}	
		} else {
			fileMap[mkdirPath] = true
		}
	}

	depLayers := make(map[int]bool)

	for _, trace := range openList {
		argsList := trace["args"].([]interface{})
		openPath := getPath(argsList)
		if strings.HasPrefix(openPath, dockerStorgePath) {
			regMatch := normalLayerPrefix.FindAllString(openPath, -1)
			for _, match := range regMatch {
				filePath := strings.Trim(openPath, match)
				val, isOk := t.fileUpdateRecord[`\` + filePath]
				if isOk {
					depLayers[val] = true
				}
			}
			regMatch = totalLayerPrefix.FindAllString(openPath, -1)
			for _, match := range regMatch{
				openMode := getMode(argsList)
				// write file
				if strings.Contains(openMode, "O_WRONLY") || strings.Contains(openMode, "O_RDWR") {
					filePath := strings.Trim(openPath, match)
					t.fileUpdateRecord[`\` + filePath] = t.layerCount
				}
			}
		}
	}

	j := 0
	keys := make([]int, len(depLayers))
	for k := range depLayers {
		keys[j] = k
		j++
	}
	return keys, nil
}