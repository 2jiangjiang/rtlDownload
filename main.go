package main

import (
	"encoding/json"
	"fmt"
	"go.bug.st/serial"
	"os"
	"strconv"
	"strings"
	"time"
)

type Modem int

const (
	SOH Modem = 0x01
	STX       = 0x02
	EOT       = 0x04
	ACK       = 0x06
	NAK       = 0x15
	CAN       = 0x18
)
const (
	blockSize = 0x400
)

var (
	bandrates = []int{
		0:  0,
		1:  110,
		2:  300,
		3:  600,
		4:  1200,
		5:  2400,
		6:  4800,
		7:  9600,
		8:  14400,
		9:  19200,
		10: 38400,
		11: 56000,
		12: 57600,
		13: 115200,
		14: 128000,
		15: 153600,
		16: 230400,
		17: 380400,
		18: 460800,
		19: 500000,
		20: 921600,
		21: 1000000,
		22: 1382400,
		23: 1444400,
		24: 1500000,
	}
)

func addressToBytes(u uint32) []byte {
	d := make([]byte, 4)
	d[0] = byte(u) & 0xff
	d[1] = byte(u>>8) & 0xff
	d[2] = byte(u>>16) & 0xff
	d[3] = byte(u>>24) & 0xff
	return d
}
func addressTo3Bytes(u uint32) []byte {
	d := make([]byte, 3)
	d[0] = byte(u) & 0xff
	d[1] = byte(u>>8) & 0xff
	d[2] = byte(u>>16) & 0xff
	return d
}
func sizeTo2Bytes(size uint32) []byte {
	size = (size + 4095) / 4096
	d := make([]byte, 2)
	d[0] = byte(size) & 0xff
	d[1] = byte(size>>8) & 0xff
	return d
}

func xModem1K(port serial.Port, address uint32, data []byte, packet *int) {
	buf := make([]byte, 1)
	port.ResetInputBuffer()
	for len(data) > 0 {
		sum := byte(0xff)
		send := data[0:min(len(data), blockSize)]
		if len(send) < blockSize {
			send = append(send, make([]byte, blockSize-len(send))...)
			for a := len(data); a < len(send); a++ {
				send[a] = 0xff
			}
		}
		sendBlock := append(append([]byte{STX, byte(*packet), byte(*packet) ^ 0xff}, addressToBytes(address)...), send...)
		for _, d := range sendBlock {
			sum += d
			//fmt.Printf("%02X ", d)
		}
		port.Write(sendBlock)
		port.Write([]byte{sum})
		port.Drain()
		port.Read(buf)
		//fmt.Printf("sum:%02X\n", sum)
		if buf[0] == ACK {
			//fmt.Printf("send %d\n", *packet)
			data = data[min(len(data), blockSize):]
			address += uint32(blockSize)
			*packet++
		} else if buf[0] == NAK {
			//fmt.Printf("resend %d\n", *packet)
		}
	}
}

func Command(s serial.Port, command []byte) {
	buf := make([]byte, 1)
	s.Write(command)
	s.Drain()
	//fmt.Println("Command:", command)
	s.Read(buf)
	for buf[0] != ACK && buf[0] != 0x27 {
		s.Read(buf)
		//fmt.Println(buf)
		if buf[0] > 0x1F && buf[0] < 0x7F {
			fmt.Print(string(buf[0]))
		}
	}
	if buf[0] == 0x27 {
		s.Read(buf)
		fmt.Print(string(buf[0]))
		s.Read(buf)
		fmt.Print(string(buf[0]))
		s.Read(buf)
		fmt.Print(string(buf[0]))
		s.Read(buf)
		fmt.Print(string(buf[0]))
	}
	//fmt.Println()
	time.Sleep(time.Millisecond * 50)
}
func Write(s serial.Port, bp int, head uint32, headData []byte, address []uint32, datas [][]byte) {
	packet := 1
	Command(s, []byte{0x05, byte(bp)})
	s.SetMode(&serial.Mode{BaudRate: bandrates[bp]})
	Command(s, []byte{0x07})
	xModem1K(s, head, headData, &packet)
	Command(s, []byte{EOT})
	s.SetMode(&serial.Mode{BaudRate: 115200})
	Command(s, []byte{0x26, 0x01, 0x01, 0x00})
	for i, d := range datas {
		Command(s, append(append([]byte{0x17}, addressTo3Bytes(address[i])...), sizeTo2Bytes(uint32(len(d)))...))
	}
	packet = 1
	Command(s, []byte{0x05, byte(bp)})
	s.SetMode(&serial.Mode{BaudRate: bandrates[bp]})
	Command(s, []byte{0x07})
	for p, data := range datas {
		xModem1K(s, address[p], data, &packet)
	}
	Command(s, []byte{EOT})
	s.SetMode(&serial.Mode{BaudRate: 115200})
	for i := 0; i < len(address); i++ {
		Command(s, append(append([]byte{0x27}, addressTo3Bytes(address[i])...), addressTo3Bytes(uint32(len(datas[i])))...))
	}
}
func main() {
	bootA := 0x00082000
	var bootP string
	var images []string
	var addresses []uint32
	var BaudRate int
	var dev string
	var confP string
	var monitor bool
	fmt.Println("start")
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-h", "--help":
			i++
			fmt.Println("usage: rtlDownload -c config -d tty/COM")
			fmt.Println("       rtlDownload tty/COM path_to_imgtool_flashloader_amebad.bin BaudRate 0x08000000 km0_boot_all.bin 0x08004000 km4_boot_all.bin 0x08006000 km0_km4_image2.bin...")
			fmt.Println("config:{")
			fmt.Println("			\"BrandRate\":1500000,")
			fmt.Println("			\"flashloader\":{")
			fmt.Println("				\"path\":\"path_to_imgtool_flashloader_amebad.bin\",")
			fmt.Println("				\"address\":\"0x00082000\"},")
			fmt.Println("			\"img1\":{")
			fmt.Println("				\"path\":\"km0_boot_all.bin\",")
			fmt.Println("				\"address\":\"0x08000000\"},")
			fmt.Println("			...,")
			fmt.Println("		}")
		case "-v", "-version":
			i++
			fmt.Println("rtlDownload version 0.1")
		case "-c", "--config":
			i++
			confP = os.Args[i][:max(0, strings.LastIndex(os.Args[i], "/"))]
			j, _ := os.ReadFile(os.Args[i])
			config := make(map[string]any)
			err := json.Unmarshal(j, &config)
			if err != nil {
				fmt.Println("err:", err)
				return
			}
			for k, v := range config {
				if k == "BrandRate" {
					BaudRate = int(v.(float64))
				} else if k == "flashloader" {
					bootP = v.(map[string]any)["path"].(string)
					b, _ := strconv.ParseUint(v.(map[string]any)["address"].(string)[2:], 16, 32)
					bootA = int(b)
				} else {
					images = append(images, v.(map[string]any)["path"].(string))
					b, _ := strconv.ParseUint(v.(map[string]any)["address"].(string)[2:], 16, 32)
					addresses = append(addresses, uint32(b))
				}
			}
		case "-d", "--device":
			i++
			dev = os.Args[i]
		case "-b", "--baudrate":
			i++
			BaudRate, _ = strconv.Atoi(os.Args[i])
		case "-m", "--monitor":
			monitor = true
		default:
			dev = os.Args[i]
			i++
			bootP = os.Args[i]
			i++
			BaudRate, _ = strconv.Atoi(os.Args[i])
			i++
			for i < len(os.Args) {
				b, _ := strconv.ParseUint(os.Args[i][2:], 16, 32)
				addresses = append(addresses, uint32(b))
				i++
				images = append(images, os.Args[i])
				i++
			}
		}
	}
	if dev == "" {
		fmt.Println("err: device not specified")
	}
	fmt.Println("Dir:", confP)
	fmt.Println("Device:", dev, "BrandRate:", BaudRate)
	fmt.Printf("boot:%s	address:0x%08X\n", bootP, bootA)
	for i := range addresses {
		fmt.Printf("part:%s	address:0x%08X\n", images[i], addresses[i])
	}
	s, _ := serial.Open(dev, &serial.Mode{BaudRate: 115200, DataBits: 8})
	defer s.Close()
	bootD, _ := os.ReadFile(confP + "/" + bootP)
	var imgs [][]byte
	for _, i := range images {
		di, _ := os.ReadFile(confP + "/" + i)
		imgs = append(imgs, di)
	}
	bp := 13
	for p, b := range bandrates {
		if BaudRate == b {
			bp = p
		}
	}
	s.SetDTR(false)
	s.SetDTR(true)
	s.SetRTS(true)
	time.Sleep(time.Millisecond * 10)
	s.SetRTS(false)
	s.SetDTR(false)
	time.Sleep(time.Millisecond * 100)
	buf := make([]byte, 32)
	s.Read(buf)
	fmt.Println(string(buf))
	Write(s, bp, uint32(bootA), bootD, addresses, imgs)
	s.SetRTS(true)
	time.Sleep(time.Millisecond * 10)
	s.SetRTS(false)
	s.SetDTR(false)
	buf = buf[:1]
	sbuf := make([]byte, 1)
	go func() {
		for {
			os.Stdin.Read(sbuf)
			s.Write(sbuf)
		}
	}()
	for monitor == true {
		s.Read(buf)
		fmt.Print(string(buf))
	}
}
