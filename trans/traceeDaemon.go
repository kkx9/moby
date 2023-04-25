package main
import (
	"fmt"
	"net"
	"os/exec"
	_"os"
)

func main() {
	ln, err := net.ListenUDP("udp", &net.UDPAddr{
		IP: net.IPv4(172, 17, 0, 1),
		Port: 11451,
	})
	if err != nil {
		fmt.Println(err)
		return 
	}
	defer ln.Close()
	for {
		var data [1024]byte
		n, addr, err := ln.ReadFromUDP(data[:])
		if err != nil {
			fmt.Println("read fail:", err)
			continue
		}
		fmt.Println("rece message")
		if com := string(data[:n]); com == "Close" {
			cmd := exec.Command("/bin/bash","-c",`docker stop tracee && docker rm tracee`)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println("close fail:",err, out)
				return
			}
			fmt.Println("Close tracee")
		} else if com == "Open" {
			cmd := exec.Command("/bin/bash","-c",`docker run  --name tracee -d  --pid=host --cgroupns=host --privileged  -v /etc/os-release:/etc/os-release-host:ro -v /home/copyright/tracee_test:/tmp  -e LIBBPFGO_OSRELEASE_FILE=/etc/os-release-host aquasec/tracee:0.9.3 trace --trace container --trace event=security_file_open,open,openat --trace open.pathname!="/var/run/docker/runtime-runc/*" --trace openat.pathname!="/var/run/docker/runtime-runc/*" --trace security_file_open.pathname!="/var/run/docker/runtime-runc/*" --trace open.pathname!="/var/lib/docker/aufs/diff/*" --trace openat.pathname!="/var/lib/docker/aufs/diff/*" --trace security_file_open.pathname!="/var/lib/docker/aufs/diff/*" --output out-file:/tmp/tracee.log --output json`)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println("open fail:",err, string(out))
				// return
			}
			fmt.Println("Open tracee")
		} else if com == "Clear" {
			cmd := exec.Command("/bin/bash","-c",`rm -rf /home/copyright/tracee_test/*`)
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println("clear fail:",err, out)
				return
			}
			fmt.Println("Clear tracee")
		}

		_, err = ln.WriteToUDP([]byte("Done"), addr)
		if err != nil {
			fmt.Println("write fail:", err)
		}

	}

	// cmd := exec.Command("/bin/bash","-c",`docker run  --name tracee --rm -d  --pid=host --cgroupns=host --privileged  -v /etc/os-release:/etc/os-release-host:ro -v /home/copyright/tracee_test:/tmp  -e LIBBPFGO_OSRELEASE_FILE=/etc/os-release-host aquasec/tracee:0.9.3 trace --trace container --trace event=security_file_open,open,openat --trace open.pathname!="/var/run/docker/runtime-runc/*" --trace openat.pathname!="/var/run/docker/runtime-runc/*" --trace security_file_open.pathname!="/var/run/docker/runtime-runc/*" --trace open.pathname!="/var/lib/docker/aufs/diff/*" --trace openat.pathname!="/var/lib/docker/aufs/diff/*" --trace security_file_open.pathname!="/var/lib/docker/aufs/diff/*" --output out-file:/tmp/tracee.log --output json`)
	// err := cmd.Run()
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// fmt.Println("Open tracee")
}