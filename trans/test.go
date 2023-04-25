package main
import (
	"fmt"
	"os"
	// "strings"
	// "golang.org/x/sys/unix"
	// "github.com/moby/sys/mount"

	_ "github.com/docker/docker/image"
)

func main() {
	// target := `/var/lib/docker/aufs/mnt/10e521fb0fccbb1465deb7f31f1cf93134d83921fce1e0d200c3901e20aadc5a`
	// data := `br:/var/lib/docker/aufs/diff/10e521fb0fccbb1465deb7f31f1cf93134d83921fce1e0d200c3901e20aadc5a=rw:/var/lib/docker/aufs/diff/e3301ec5fc9e5dc87761b4059109930ba6cde3ac69e5a187537e8a497d7d99e0=ro+wh:/var/lib/docker/aufs/diff/757763c0fe12a2150cc89a4265b76f66c1fca26f612995132518604cafbc9c70=ro+wh:/var/lib/docker/aufs/diff/2644366381ab631fe4efd274ead854dcb66d7b32d3c76d2888a99d914ed786b7=ro+wh:/var/lib/docker/aufs/diff/1fd5655ea6d730da6d00ea617f58b2ffb4b9294a8ffb5e56880880e7fa1ad212=ro+wh:/var/lib/docker/aufs/diff/e3301ec5fc9e5dc87761b4059109930ba6cde3ac69e5a187537e8a497d7d99e0=ro+wh,dio,xino=/dev/shm/aufs.xino,dirperm1`
	// data := `br:/var/lib/docker/aufs/diff/10e521fb0fccbb1465deb7f31f1cf93134d83921fce1e0d200c3901e20aadc5a=rw:/var/lib/docker/aufs/diff/e3301ec5fc9e5dc87761b4059109930ba6cde3ac69e5a187537e8a497d7d99e0=ro+wh:/var/lib/docker/aufs/diff/757763c0fe12a2150cc89a4265b76f66c1fca26f612995132518604cafbc9c70=ro+wh:/var/lib/docker/aufs/diff/2644366381ab631fe4efd274ead854dcb66d7b32d3c76d2888a99d914ed786b7=ro+wh:/var/lib/docker/aufs/diff/1fd5655ea6d730da6d00ea617f58b2ffb4b9294a8ffb5e56880880e7fa1ad212=ro+wh:/var/lib/docker/aufs/diff/e3301ec5fc9e5dc87761b4059109930ba6cde3ac69e5a187537e8a497d7d99e0=ro+wh,dio,xino=/dev/shm/aufs.xino,dirperm1`
	// err := unix.Mount("none", target, "aufs", 0, data)
	// fmt.Println(err)
	// mount.Unmount(target)
	err := os.Remove("/tracee/tracee.log")
	if err != nil {
		fmt.Println(err)
	}
}