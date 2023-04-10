package depcache

import (
	"crypto/sha256"
	"sort"
	"strings"
	"fmt"
	"os"
	"bufio"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/sirupsen/logrus"
)

type Depcache struct {
	cacheMap		map[string][]string
	cacheIDMap		map[string]string
	configMap		map[string]*containertypes.Config
	hackImageSlice  [][2]string
	store 			image.Store
	hackMap			map[string]bool
	hackHash		map[string]string
}

func NewDepcache(store image.Store) *Depcache {
	c := &Depcache{
		cacheMap:		make(map[string][]string),
		cacheIDMap:		make(map[string]string),
		configMap:  	make(map[string]*containertypes.Config),
		store:			store,
		hackImageSlice:	[][2]string{},
		hackMap:		make(map[string]bool),
		hackHash:		make(map[string]string),
	}
	return c
}

func (d *Depcache) depLayerHash(layers []string) string {
	sort.Strings(layers)
	layerString := strings.Join(layers, "")
	return fmt.Sprintf("%x",sha256.Sum256([]byte(layerString)))
}

func (d *Depcache) AddLayer(layerDigest string, depLayers []string, config *containertypes.Config, cacheID string) {
	// d.unhackImage()
	depHash := d.depLayerHash(depLayers)
	d.cacheIDMap[layerDigest] = cacheID
	for _,v:=range d.cacheMap[depHash] {
		if compare(d.configMap[v], config) {
			logrus.Debug("[Dep cache]Add dup layer")
			return
		}
	}
	d.cacheMap[depHash] = append(d.cacheMap[depHash], layerDigest)
	d.configMap[layerDigest] = config
} 

func (d *Depcache) CheckLayer(config *containertypes.Config, depLayers []string, imageLayers *[]layer.DiffID, cacheIDList *[]string) string {
	// d.unhackImage()
	depHash := d.depLayerHash(depLayers)
	for _,v:=range d.cacheMap[depHash] {
		logrus.Debug(v)
		logrus.Debug(d.configMap[v])
		if compare(d.configMap[v], config) {
			if _, err := d.store.Get(image.ID(v)); err == nil {
				*cacheIDList = append(*cacheIDList, d.cacheIDMap[v])
				d.hackImage(v, imageLayers, cacheIDList)
				return v
			} else{
				logrus.Debug(err)
			}
		}
	}
	return ""
}

func(d *Depcache) hackImage(layerDigest string, imageLayers *[]layer.DiffID, cacheIDList *[]string) {
	logrus.Debugf("hack image info: %s", layerDigest)
	d.hackLayers(cacheIDList)
	for _,v := range *cacheIDList {
		logrus.Debugf("cache id:%s", v)
	}
	imageFile := `/var/lib/docker/image/aufs/imagedb/content/sha256/` + strings.TrimPrefix(layerDigest, "sha256:")
	archFile := imageFile + ".arch"
	content, err := os.ReadFile(imageFile)
	
	im, err := image.NewFromJSON(content)
	if err != nil {
		logrus.Error(err)
		return
	}

	if len(im.RootFS.DiffIDs) == len(*imageLayers) {
		for i:=0; i < len(im.RootFS.DiffIDs); i++ {
			if im.RootFS.DiffIDs[i] != (*imageLayers)[i] {
				goto diff
			}
		}
		return
	}

diff:
	err = os.Rename(imageFile, archFile)
	if err != nil {
		logrus.Error(err)
	}
	d.hackImageSlice = append(d.hackImageSlice, [2]string{imageFile, archFile})
	if string(im.RootFS.DepChainID) == "" {
		im.RootFS.DepChainID = layer.CreateChainID(im.RootFS.DiffIDs)

	}
	logrus.Debug(im.RootFS.DiffIDs)
	logrus.Debug(*imageLayers)

	for i := 0; i < len(*imageLayers) && i < len(im.RootFS.DiffIDs); i++ {
		im.RootFS.DiffIDs[i] = (*imageLayers)[i]
	}

	newContent, err := im.MarshalJSON()
	os.WriteFile(imageFile, newContent, os.ModePerm)
}

func(d *Depcache) hackLayers(cacheIDList *[]string) {
	//first layer
	if len(*cacheIDList) == 1{
		cacheID := (*cacheIDList)[0]
		content, err := os.ReadFile(`/var/lib/docker/aufs/layers/` + strings.TrimPrefix(cacheID, "sha256:"))
		(*cacheIDList) = (*cacheIDList)[:0]
		if err != nil {
			logrus.Debugf("hack layers error: ", err)
		} else {
			ll := strings.Split(string(content), "\n")
			for i:=0; i < len(ll); i++ {
				if len(ll[i]) > 0 {
					*cacheIDList = append(*cacheIDList, ll[i])
				}
			}
		}
		*cacheIDList = append(*cacheIDList, cacheID)
		return
	}
	outSlice := []string{}
	for _,v := range *cacheIDList {
		hash := d.depLayerHash(outSlice)
		if d.hackMap[v] == false || d.hackHash[v] != hash {
			layerFile := `/var/lib/docker/aufs/layers/` + strings.TrimPrefix(v, "sha256:")
			logrus.Debug(layerFile)

			content, err := os.ReadFile(layerFile)
			if err != nil {
				fmt.Println(err)
			}
			logrus.Debug("arch content:",(string(content)))
			
			archFile := layerFile + ".arch"

			err = os.Rename(layerFile, archFile)
			if err != nil {
				logrus.Error(err)
			}
			file, err := os.OpenFile(layerFile, os.O_CREATE|os.O_WRONLY, os.ModePerm)
			if err != nil {
				logrus.Error(err)
			}
			datawriter := bufio.NewWriter(file)
			for i:=len(outSlice) - 1; i>=0; i-- {
				logrus.Debug(outSlice[i])
				_, _ = datawriter.WriteString(outSlice[i] + "\n")
			}
			datawriter.Flush()
			file.Close()
			d.hackMap[v] = true
			d.hackHash[v] = hash
		}
		outSlice = append(outSlice, v)
	}
}

func (d *Depcache) unhackImage() {
	for _,v := range d.hackImageSlice{
		os.Remove(v[0])
		os.Rename(v[1], v[0])
	}
	d.hackImageSlice = d.hackImageSlice[:0]
}