package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

var (
	port    = flag.String("p", "1025", "默认监听端口")
	timeout = flag.Duration("t", 10*time.Second, "连接延迟超时")
	//wg sync.WaitGroup
)

func Banner() {

	fmt.Println(`
           __.__  .__       _______   
__  _  __ |__|  | |__| ____ \   _  \  
\ \/ \/ / |  |  | |  |/    \/  /_\  \ 
 \     /  |  |  |_|  |   |  \  \_/   \
  \/\_/\__|  |____/__|___|  /\_____  /
      \______|            \/       \/ 
        go-proxy-socks `)
}

func main() {
	Banner()
	flag.Parse()
	//fmt.Println(*timeout)
	checkArgs()
	server, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		fmt.Printf("Listen failed: %v\n", err)
		return
	}
	fmt.Println("Listen success")
	for {
		client, err := server.Accept()
		fmt.Println("Receive a connect")
		if err != nil {
			fmt.Printf("Accept failed: %v", err)
			continue
		}
		go handleConnection(client)
	}
}

func checkArgs() {
	if int(*timeout) < 5000000000 {
		fmt.Println("error: 延迟时间时间不能小于5s")
		os.Exit(0)
	}
}

func handleConnection(client net.Conn) {
	defer client.Close()
	v, err := checkSocksVersion(client)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	switch v {
	case 5:
		err := socks5(client)
		if err != nil {
			fmt.Println("连接出现错误:", err.Error())
			return
		}
	case 4:
		err := socks4(client)
		if err != nil {
			fmt.Println("连接出现错误:", err.Error())
			return
		}
	default:

	}
}

func checkSocksVersion(client net.Conn) (version int, err error) {
	b := make([]byte, 256)
	_, err = io.ReadFull(client, b[:2])
	//fmt.Println(b)
	if err != nil {
		return 0, errors.New("reading header: " + err.Error())
	}

	version, nNum := int(b[0]), int(b[1])
	// 读取 METHODS 列表

	_, err = io.ReadFull(client, b[:nNum])
	if err != nil {
		return 0, errors.New("reading methods: " + err.Error())
	}
	r := false
	for _, c := range b[:nNum] {
		if c == 0x00 {
			r = true
		}
	}
	if !r {
		return 0, errors.New("error method: " + err.Error())
	}

	switch version {
	case 4:
		_, err = client.Write([]byte{0x04, 0x00})

	case 5:
		_, err = client.Write([]byte{0x05, 0x00})
	default:
		err = errors.New(strconv.Itoa(version))
	}
	if err != nil {
		return 0, err
	}

	return version, nil
}

func socks4(client net.Conn) (err error) {
	return
}

func socks5(client net.Conn) (err error) {

	conn, err := Socks5Connect(client)
	if err != nil {
		return
	}
	f := func(dst, src net.Conn) {
		defer dst.Close()
		defer src.Close()
		go io.Copy(dst, src)
		go io.Copy(src, dst)
	}
	f(client, conn)
	//go io.Copy(conn, client)
	fmt.Println(client.RemoteAddr(), "->", client.LocalAddr(), "->", conn.RemoteAddr())
	return
}

func Socks5Connect(client net.Conn) (dest net.Conn, err error) {
	b := make([]byte, 256)
	_, err = io.ReadFull(client, b[:4])
	//fmt.Println(b)
	if err != nil {
		return nil, errors.New("reading error " + err.Error())
	}

	version, cmd, rsv, atyp := b[0], b[1], b[2], b[3]
	if version != 5 || cmd != 1 {
		return nil, errors.New("invalid ver/cmd")
	}
	// 目的地址类型 1:ipv4, 3:域名, 4:ipv6
	addr := ""
	switch atyp {
	case 1:
		_, err := io.ReadFull(client, b[:4])
		if err != nil {
			return nil, errors.New("invalid ipv4: " + err.Error())
		}
		addr = fmt.Sprintf("%v.%v.%v.%v", b[0], b[1], b[2], b[3])
	case 3:
		_, err := io.ReadFull(client, b[:1])
		if err != nil {
			return nil, err
		}
		domainNum := int(b[0])
		_, err = io.ReadFull(client, b[:domainNum])
		if err != nil {
			return nil, errors.New("invalid domain: " + err.Error())
		}
		addr = string(b[:domainNum])
	case 4:
		return nil, errors.New("invalid ipv6: not support")
	default:
		return nil, errors.New("invalid atyp")
	}
	_, err = io.ReadFull(client, b[:2])

	p := b[:2]
	destAddPort := addr + ":" + strconv.Itoa(int(binary.BigEndian.Uint16(p)))
	//fmt.Println(destAddPort)
	dest, err = net.DialTimeout("tcp", destAddPort, *timeout)
	if err != nil {
		return nil, errors.New("dial dst: " + err.Error())
	}
	rb := []byte{version, 0, rsv, atyp}
	if atyp == 3 {
		n := len(addr)
		rb = append(rb, byte(n))
	}
	for _, a := range addr {
		rb = append(rb, byte(a))
	}
	for _, i := range p {
		rb = append(rb, i)
	}
	//fmt.Println(rb)
	_, err = client.Write(rb)
	if err != nil {
		dest.Close()
		return nil, errors.New("write error: " + err.Error())
	}
	return dest, err

}
