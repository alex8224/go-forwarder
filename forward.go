package main

import (
	"flag"
	"fmt"
	"log"
	"net"
)

var listen = flag.String("listenport", ":1088", "Listen Port,listen host:port")
var destHost = flag.String("toHost", "", "Destination Host forware to, host:port")

func bridge(source, dest net.Conn) {

	buff := make([]byte, 131072)
	for {
		nbytes, err := source.Read(buff)
		if err != nil {
			fmt.Printf("read bytes from %s err, desc: %s\n", source.RemoteAddr(), err)
			source.Close()
			break
		}

		wbytes, werr := dest.Write(buff[0:nbytes])
		if werr != nil {
			fmt.Printf("write bytes to %s err, desc: %s\n", dest.RemoteAddr(), err)
			dest.Close()
			break
		} else {
			fmt.Printf("write %d bytes to %s\n", wbytes, dest.RemoteAddr())
		}
	}
}

func handleConnection(client net.Conn) {
	fmt.Println("toHost", *destHost)
	remoteSocket, err := net.Dial("tcp", *destHost)
	if err != nil {
		fmt.Printf("connect to %s failed, desc: %s", *destHost, err)
		return
	}

	go bridge(client, remoteSocket)
	go bridge(remoteSocket, client)
}

func main() {

	flag.Parse()

	l, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		serversocket, serr := l.Accept()
		if serr != nil {
			fmt.Println("accept err", serr)
			continue
		}

		fmt.Println("accept from", serversocket)

		go handleConnection(serversocket)
	}
}
