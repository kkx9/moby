package main
import (
	"net"
	"fmt"
)

func main() {
	socket, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP: net.IPv4(172, 17, 0, 1),
		Port: 11451,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer socket.Close()
	// sendData := []byte("Close")
	// _, err = socket.Write(sendData)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	sendData := []byte("Open")
	_, err = socket.Write(sendData)
	if err != nil {
		fmt.Println(err)
		return
	}
	data := make([]byte, 100)
	n, _, err := socket.ReadFromUDP(data)
	if err != nil {
		fmt.Println("fail rece:", err)
		return
	}
	fmt.Println("rece:", string(data[:n]))

	sendData = []byte("Close")
	_, err = socket.Write(sendData)
	if err != nil {
		fmt.Println(err)
		return
	}

	n, _, err = socket.ReadFromUDP(data)
	if err != nil {
		fmt.Println("fail rece:", err)
		return
	}
	fmt.Println("rece:", string(data[:n]))

	sendData = []byte("Clear")
	_, err = socket.Write(sendData)
	if err != nil {
		fmt.Println(err)
		return
	}

	n, _, err = socket.ReadFromUDP(data)
	if err != nil {
		fmt.Println("fail rece:", err)
		return
	}
	fmt.Println("rece:", string(data[:n]))

	sendData = []byte("Open")
	_, err = socket.Write(sendData)
	if err != nil {
		fmt.Println(err)
		return
	}

	n, _, err = socket.ReadFromUDP(data)
	if err != nil {
		fmt.Println("fail rece:", err)
		return
	}
	fmt.Println("rece:", string(data[:n]))

	sendData = []byte("Close")
	_, err = socket.Write(sendData)
	if err != nil {
		fmt.Println(err)
		return
	}

	n, _, err = socket.ReadFromUDP(data)
	if err != nil {
		fmt.Println("fail rece:", err)
		return
	}
	fmt.Println("rece:", string(data[:n]))

}