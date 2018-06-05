package main

/*
#include<stdlib.h>
#include<stdio.h>
#include<stdlib.h>
#include <sys/types.h>
#include <arpa/inet.h>
#include<netinet/in.h>
#include <sys/socket.h>
#include <linux/if.h>
#include <linux/netfilter_ipv4.h>
#include <linux/netfilter_ipv6/ip6_tables.h>
#define SOL_IP 0
char*
getdestaddr(int fd)
{
    struct sockaddr_in *destaddr = (struct sockaddr_in*)malloc(sizeof(struct sockaddr_in));
    socklen_t socklen = sizeof(*destaddr);
    int error = 0;

    error = getsockopt(fd, SOL_IP, SO_ORIGINAL_DST, destaddr, &socklen);
    if (error) {
        printf("getsockopt failed:%d\n", error);
        return NULL;
    }

    uint16_t port = ntohs(destaddr->sin_port);
    char IPADDR[16];
    inet_ntop(AF_INET, &destaddr->sin_addr, IPADDR, 100);
    char* outbuff = (char*)malloc(21);
    if(outbuff != NULL) {
      snprintf(outbuff, 21, "%s:%d", IPADDR, port);
    }
    if(destaddr != NULL) {
        free(destaddr);
    }
    return outbuff;
}
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"flag"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type destHost struct {
	tcpConn  *net.TCPConn
	destAddr string
}

var socks5_host = flag.String("socks5", "", "host:port")
var listen = flag.String("listen", "", "host:port")

const (
	VER        = 0x05
	NO_AUTH    = 0x00
	METHOD_LEN = 0x01
	CONNECT    = 0x01
	RSV        = 0x00
	ATYP       = 0x01
	CMD_OK     = 0x00
	CMD_FAIL   = 0xff
	POOL_SIZE  = 2
)

var (
	SOCKS5_HELLO = []byte{VER, METHOD_LEN, NO_AUTH}
	SOCKS5_REQ   = []byte{VER, CONNECT, RSV, ATYP}
)

func socks5_talk(conn *net.TCPConn, destaddr string) {
	dest, err := net.Dial("tcp", *socks5_host)
	if err != nil {
		log.Printf("%s", err)
		return
	}

	destTcp, ok := dest.(*net.TCPConn)
	if !ok {
		log.Printf("assert failed %s\n", err)
		return
	}

	buff := bytes.NewBuffer(SOCKS5_HELLO)
	hbytes, err := buff.WriteTo(dest)

	if err != nil {
		log.Printf("socks5 failed %s, %d\n", err, hbytes)
		return
	}

	resp := make([]byte, 2)
	rbytes, err := dest.Read(resp)
	if err != nil {
		log.Printf("socks5 failed %s, rbytes:%d\n", err, rbytes)
		return
	}

	if resp[0] != VER || resp[1] == CMD_FAIL {
		log.Printf("socks5 response protocol error")
		return
	}

	truedest := strings.Split(destaddr, ":")
	ipbytes := net.ParseIP(truedest[0])
	port, _ := strconv.Atoi(truedest[1])
	portbytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portbytes, uint16(port))
	buff.Write(SOCKS5_REQ)
	buff.Write(ipbytes[12:])
	buff.Write(portbytes)
	buff.WriteTo(dest)

	reqbytes := make([]byte, 10)
	req, err := dest.Read(reqbytes)
	if err != nil {
		log.Printf("socks5 rep failed, %s,req:%d\n", err, req)
		return
	}

	if reqbytes[1] != CMD_OK {
		log.Printf("socks5 connect failed\n")
		return
	}

	log.Printf("socks5 ok, start forward from %s to %s\n", conn.RemoteAddr(), destaddr)
	go bridge(conn, destTcp, "src")
	go bridge(destTcp, conn, "dest")
}

func bridge(src, dest *net.TCPConn, name string) {
	defer func() {
		src.Close()
		if name == "dest" {
			dest.CloseRead()
		}
	}()
	_, err := io.Copy(dest, src)
	if err != nil {
		log.Printf("close forward direction: %s,%s\n", name, err)
		return
	}
}

func acceptor(listener net.Listener, socket chan destHost, index int) {

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error %s\n", err)
			continue
		}

		tcpsock, destaddr := findNatDest(conn)
		if tcpsock == nil {
			log.Println(destaddr)
			continue
		}
		socket <- destHost{tcpsock, destaddr}
		log.Printf("acceptor %d accept a conn %s\n", index, tcpsock.RemoteAddr())
	}
}

func findNatDest(conn net.Conn) (src *net.TCPConn, destaddr string) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		log.Fatal("type error")
	}

	tcpFile, error := tcpConn.File()

	if error != nil {
		log.Println(error)
		return nil, "tcpfile no fd"
	}

	fd := tcpFile.Fd()

	ret := C.getdestaddr(C.int(fd))
	if ret == nil {
		return nil, "nat dest not found"
	}
	addr := C.GoString(ret)
	C.free(unsafe.Pointer(ret))
	cerr := tcpFile.Close()
	if cerr != nil {
		log.Printf("close file failed, reason:%s", cerr)
		return nil, "close tcpfile failed"
	}

	return tcpConn, addr
}

func start_forever() {
	listener, error := net.Listen("tcp", *listen)
	if error != nil {
		log.Fatal(error)
	}

	acceptor_pool := make(chan destHost)

	for i := 0; i < POOL_SIZE; i++ {
		go acceptor(listener, acceptor_pool, i)
	}

	for {
		select {
		case natHost := <-acceptor_pool:
			go socks5_talk(natHost.tcpConn, natHost.destAddr)
		case <-time.After(1 * time.Second):
		}
	}

}

func main() {
	flag.Parse()
	if *socks5_host == "" || *listen == "" {
		flag.Usage()
		return
	}

	start_forever()
}
