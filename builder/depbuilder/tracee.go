package depbuilder // import "github.com/docker/docker/builder/depbuilder"

import (
	"os"
	"strings"
	"bufio"
	"regexp"
	_ "strconv"
	"time"
	"encoding/json"
	"io/ioutil"
	"net"
	"context"

	"github.com/sirupsen/logrus"
)

type Tracee struct {
	traceLog 			*os.File
	lastTime			int64
	layerDigest			string
	lastCacheID			string
	LayerCount			int
	LayerDict			map[int]string
	fileUpdateRecord	map[string]int
	traceRecord			[]map[string]interface{}
}

const dockerStorgePath = "/var/lib/docker/aufs/diff/"

var dockerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/((aufs)|(vfs)|(overlay))/dir/`)

var initLayerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/((aufs)|(vfs)|(overlay))/dir/[0-9a-zA-Z]+-init/`)

var normalLayerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/((aufs)|(vfs)|(overlay))/dir/[0-9a-zA-Z]+/`)

var totalLayerPrefix = regexp.MustCompile(`(?m)^/var/lib/docker/((aufs)|(vfs)|(overlay))/dir/[0-9a-zA-Z]+(-init)`)

const normalLayerReg = `(?m)^/var/lib/docker/aufs/dir/`

const writeFlag = (os.O_WRONLY | os.O_CREATE)

const readWriteFlag = (os.O_RDWR | os.O_APPEND)



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

func walkDir(path string, files []string) ([]string, error) {
	dir, err := ioutil.ReadDir(path)
	if err != nil {
		return files, err
	}

	sep := string(os.PathSeparator)

	for _, fi := range dir {
		if fi.IsDir() {
			files = append(files, path + sep + fi.Name())
			files, err = walkDir(path + sep + fi.Name(), files)
		} else {
			files = append(files, path + sep + fi.Name())
		}
	}

	return files, nil
}

func (t *Tracee) CallTracee(com string) {
	socket, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP: net.IPv4(172, 17, 0, 1),
		Port: 11451,
	})
	if err != nil {
		logrus.Error("fail connect:", err)
		return
	}
	defer socket.Close()
	sendData := []byte(com)
	_, err = socket.Write(sendData)
	if err != nil {
		logrus.Error("fail send:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second * 10))
	defer cancel()
	ch := make(chan struct{}, 0)
	go func() {
		data := make([]byte, 100)
		_, _, err = socket.ReadFromUDP(data)
		if err != nil {
			logrus.Error("fail rece:", err)
			ch <- struct{}{}
			return
		}
		// logrus.Debug("rece:", string(data[:n]))
		ch <- struct{}{}
	}()

	select {
		case <- ch:
			logrus.Debug("Done")
		case <-ctx.Done():
			logrus.Debug("Timeout")
	}
	
}

func (t *Tracee) GetStageId() string {
	for _, trace := range t.traceRecord {
		argsList := trace["args"].([]interface{})
		mkdirPath := getPath(argsList)
		if strings.Contains(mkdirPath, "-init") && strings.HasPrefix(mkdirPath, dockerStorgePath) {
			var re = regexp.MustCompile(`(?m)/[a-zA-Z0-9]*-init/`)
			for _, match := range re.FindAllString(mkdirPath, -1) {
				t.layerDigest = strings.TrimSuffix(strings.Trim(match,"//"), "-init")
				// logrus.Debugf("stage id is : %s", t.layerDigest)
				return t.layerDigest
			}
		}
	}
	return ""
}

func (t *Tracee) GetDepLayer() ([]int, []string, error) {
	rL, rD, err := t.MatchTrace()
	return rL, rD, err
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
			if strings.HasPrefix(argMap["value"].(string), `/`){
				return argMap["value"].(string)
			}
			return `/` + argMap["value"].(string)
		}
	}
	return ""
}

func getMode(argsList []interface{}) int {
	for _, arg := range argsList {
		argMap := arg.(map[string]interface{})
		if argMap["name"].(string) == "flags" {
			return int(argMap["value"].(float64))
		}
	}
	return 0
}

func (t *Tracee) checkUpdate(filePath string) bool {
	fileStat, err := os.Stat(filePath)
	if err != nil {
		// logrus.Debugf("can not find file: %s %s", openPath, strings.Replace(openPath, t.layerDigest, t.lastCacheID, -1))
		return false
	} else if !fileStat.ModTime().Before(time.Unix(0, t.lastTime-t.lastTime%1000000000)){
		// logrus.Debugf("file be updated: %s %s", filePath, fileStat.ModTime())
		return true
	} else {
		// logrus.Debugf("file not be updated: %s %s", filePath, fileStat.ModTime())
		return false
	}
}

func checkFilepath(filePath string, fileMap map[string]bool) bool {
	// logrus.Debugf("start check file path: %s", filePath)
	if _, inMap := fileMap[filePath]; inMap {
		return true
	}
	return false
}

func (t *Tracee) checkDir(filePath string) int {
	// var re = regexp.MustCompile(`/[^/]*$`)
	// dirPath := ""
	// // logrus.Debugf("start check dir path: %s", filePath)
	// for _,match := range re.FindAllString(filePath, -1) {
	// 	// logrus.Debugf("start check dir path: %s", dirPath)
	// 	if val, inMap := t.fileUpdateRecord[dirPath]; inMap && strings.Count(dirPath, "") > 0 {
	// 		return val
	// 	}
	// 	dirPath = strings.TrimSuffix(dirPath, match)
	// }

	if val, inMap := t.fileUpdateRecord[filePath]; inMap {
		return val
	}
	return -1
}

// func HashDir(filePaths []string, cacheID string) string {
// 	sort.Slice(filePaths, func (i,j int) bool {
// 		return filePaths[i] < filePaths[j]
// 	})
// 	for _, fi := range updateFiles {
// 		filePath := strings.TrimPrefix(fi, dockerStorgePath + cacheID)
// 		info, _ := os.Stat(fi)

// 	}
// }

func (t *Tracee) MatchTrace() ([]int, []string, error) {
	var openList []map[string]interface{}
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
			return nil, nil, err
		}

		if int64(trace["timestamp"].(float64)) <= t.lastTime{
			continue
		}

		argsList := trace["args"].([]interface{})
		callPath := getPath(argsList)

		if strings.Contains(callPath, "-init") {
			continue
		}

		t.traceRecord = append(t.traceRecord, trace)

		if trace["eventName"] == "mkdir" || trace["eventName"] == "mkdirat" {
			// mkdirList = append(mkdirList, trace)
			continue
		} else {
			openList = append(openList, trace)
		}
	}

	t.CallTracee("Clear")

	logrus.Debugf("lognum is %d", len(t.traceRecord))

	t.GetStageId()

	// fileMap := make(map[string]bool)

	// for _, trace := range mkdirList {
	// 	argsList := trace["args"].([]interface{})
	// 	mkdirPath := getPath(argsList)
	// 	if strings.HasPrefix(mkdirPath, dockerStorgePath) {
	// 		//check layer
	// 		// re := regexp.MustCompile(normalLayerReg + t.layerDigest)
	// 		// if regMatch := re.FindAllString(mkdirPath, -1); len(regMatch) > 0 {
	// 		// 	pathSuffix := strings.TrimPrefix(mkdirPath, dockerStorgePath + t.layerDigest)
	// 		// 	if len(pathSuffix) > 1 {
	// 		// 		logrus.Debugf("find mkdir possible: %s", pathSuffix)
	// 		// 		if _, inMap := fileMap[pathSuffix] ; inMap {
	// 		// 			t.fileUpdateRecord[pathSuffix] = t.LayerCount
	// 		// 			logrus.Debugf("find mkdir %s", pathSuffix)
	// 		// 			delete(fileMap, pathSuffix)
	// 		// 		}
	// 		// 	}		
	// 		// }	
	// 		continue
	// 	} else if trace["processName"] == "mkdir" || trace["processName"] == "mkdirat" {
	// 		logrus.Debugf("find mkdir souce %s", mkdirPath)
	// 		t.fileUpdateRecord[mkdirPath] = t.LayerCount
	// 		// fileMap[mkdirPath] = true
	// 	}
	// }

	depLayers := make(map[int]bool)
	fileMap := make(map[string]bool)

	for _, trace := range openList {
		argsList := trace["args"].([]interface{})
		openPath := getPath(argsList)
		// logrus.Debugf("origin path: %s", openPath)
		// openMode := getMode(argsList)
		if strings.HasPrefix(openPath, dockerStorgePath) {
			// if openMode & writeFlag != 0 {
			// 	logrus.Debugf("check path: %s", openPath)
			// 	regMatch := totalLayerPrefix.FindAllString(openPath, -1)
			// 	for _, match := range regMatch{
			// 		// write file
			// 		filePath := strings.TrimPrefix(openPath, match)
			// 		if t.checkUpdate(strings.Replace(openPath, t.layerDigest, t.lastCacheID, -1)) == true {
			// 			t.fileUpdateRecord[filePath] = t.LayerCount
			// 		}
			// 	}
			// }
			
			// regMatch := normalLayerPrefix.FindAllString(openPath, -1)
			// for _, match := range regMatch {
			// 	filePath := `/` + strings.TrimPrefix(openPath, match)
			// 	val, isOk := t.fileUpdateRecord[filePath]
			// 	if isOk && val != t.LayerCount {
			// 		if checkFilepath(filePath, fileMap) == true {
			// 			depLayers[val] = true
			// 			logrus.Debugf("file dep: %s", filePath)	
			// 		}
			// 	}
			// }
			continue
		} else {
			if processName := trace["processName"].(string); !( strings.Contains(processName, "runc") || strings.Contains(processName, "docker") || strings.Contains(processName, "containerd") ) {
				fileMap[openPath] = true
				// logrus.Debugf("find open call: %s", openPath)
			} 
			val, isOk := t.fileUpdateRecord[openPath]
			if isOk {
				if checkFilepath(openPath, fileMap) == true {
					depLayers[val] = true
					// logrus.Debugf("file dep: %s", openPath)	
					// logrus.Debugf("file dep layer: %d", val)
					if dirDep := t.checkDir(openPath); dirDep != -1 {
						depLayers[dirDep] = true
						// logrus.Debugf("dir dep layer: %d", dirDep)
					}
				}
			}

			// write file
			// if openMode & writeFlag != 0 {
			// 	fullPath := dockerStorgePath + t.lastCacheID + openPath
			// 	if t.checkUpdate(fullPath) == true{
			// 		t.fileUpdateRecord[openPath] = t.LayerCount
			// 		for _, layerVal := range t.checkDir(openPath) {
			// 			depLayers[layerVal] = true
			// 		}
			// 	}
			// }
		}
	}

	var updateFiles []string
	var err error

	updateFiles, err = walkDir(dockerStorgePath + t.lastCacheID, updateFiles)

	logrus.Debugf("walk path : %s", dockerStorgePath + t.lastCacheID)

	if err == nil {
		for _, fi := range updateFiles {
			t.fileUpdateRecord[strings.TrimPrefix(fi, dockerStorgePath + t.lastCacheID)] = t.LayerCount
		}
	} else {
		return nil, nil, err
	}

	logrus.Debugf("last time %s %d", time.Unix(0, t.lastTime).Format("2006-01-02 15:04:05"), t.lastTime)

	keys := []int{}
	dL := []string{}
	for k := range depLayers {
		keys = append(keys,k)
		dL = append(dL,t.LayerDict[k])
		// logrus.Debugf("update deplayers:%d %s", k, t.LayerDict[k])
	}
	return keys, dL, nil
}