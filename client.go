package main

import (
  "github.com/vadimpilyugin/debug_print_go"
  "net"
  "os"
  "fmt"
  "io/ioutil"
  "math/rand"
  "strings"
  "time"
  "strconv"
  "encoding/binary"
  "errors"
  "path"
  "io"
)

const (
  MTU = 1460
  BUFLEN = 4096
  MAX_FN_LEN = 20
  LEN_ERR = "Filename is too long"
  FN_L = 1
  INDEX_LEN = 8
  HEADER_LEN = FN_L + MAX_FN_LEN + 2 * INDEX_LEN
  SMBUF = 256
  READY = "Ready"
  DO_RETRANSMIT = "Do another retransmission"
  FILE_RECEIVED = "File received"
  STATS = "Stats?"
  LF = '\n'
)

const (
  serverPortTcp = "8080"
  serverPortUdp = "8687"
)

type FilePart struct {
  Filename string
  PartNo int64
  NParts int64
  FilePart []byte
}

func (fp FilePart) MarshalBinary() ([]byte, error) {
  buf := make([]byte, HEADER_LEN, MTU)

  if len(fp.Filename) > MAX_FN_LEN {
    return nil, errors.New(LEN_ERR)
  }

  buf[0] = byte(len(fp.Filename))
  copy(buf[FN_L:MAX_FN_LEN+FN_L], []byte(fp.Filename))
  
  binary.PutVarint(buf[MAX_FN_LEN+FN_L:MAX_FN_LEN+FN_L+INDEX_LEN], fp.PartNo)
  binary.PutVarint(buf[MAX_FN_LEN+FN_L+INDEX_LEN:MAX_FN_LEN+FN_L+2*INDEX_LEN], fp.NParts)
  
  buf = append(buf, fp.FilePart...)
  return buf, nil
}

func randomPartsSeq(nParts int64) []int64 {
  ar := make([]int64, 0, nParts)
  var i int64
  for i = 0; i < nParts; i++ {
    ar = append(ar, i)
  }
  rand.Shuffle(len(ar), func(i, j int) {
    ar[i], ar[j] = ar[j], ar[i]
  })
  return ar
}

func sendFile(fileToSend string, partLen int, conn net.Conn) {
  file, err := os.Open(fileToSend)
  if err != nil {
    printer.Fatal(err)
  }
  defer file.Close()

  content, err := ioutil.ReadAll(file)
  if err != nil {
    printer.Fatal(err)
  }

  fn := path.Base(fileToSend) + "_" + strconv.Itoa(partLen)

  fileSize := int64(len(content))
  nParts := fileSize / int64(partLen)
  if fileSize % int64(partLen) > 0 {
    nParts += 1
  }
  partsSeq := randomPartsSeq(nParts)

  done := false
  for _, partNo := range partsSeq {
    rightEnd := (partNo + 1) * int64(partLen)
    if rightEnd >= fileSize {
      rightEnd = fileSize
    }
    buf, err := FilePart{
      Filename: fn,
      PartNo: partNo,
      NParts: nParts,
      FilePart: content[partNo * int64(partLen) : rightEnd],
    }.MarshalBinary()

    if err != nil {
      printer.Fatal(err)
    }

    _, err = conn.Write(buf)
    if err != nil {
      printer.Error(err, "conn.Write")
    }
    // printer.Debug(
    //   fmt.Sprintf("payload len=%d", len(buf)), 
    //   fmt.Sprintf("%d / %d", partNo + 1, nParts),
    // )
    // if len(buf) > MTU {
    //   printer.Note("Outside of MTU limit!", fmt.Sprintf("Packet len=%d", len(buf)))
    // }

    if done {
      break
    }

    partNo++
  }
}

func readMsg(c net.Conn) []byte {
  buffer := make([]byte, SMBUF)
  n, err := c.Read(buffer)
  if err != nil && err != io.EOF {
    printer.Fatal(err)
  } else if err == io.EOF {
    printer.Fatal(err, "Client exited")
  }
  printer.Debug(buffer[:n-1], "--- server")
  return buffer[:n]
}

func sendMsg(c net.Conn, msg string) {
  _, err := c.Write([]byte(msg + "\n"))
  if err != nil {
    printer.Fatal(err)
  }
  printer.Debug(msg, "--- me")
}

func readCommand(c net.Conn, received chan string) {
  var command []byte

  for {
    for _, ch := range readMsg(c) {
      if ch == LF {
        received <- string(command)
        command = []byte("")
      } else {
        command = append(command, ch)
      }
    }
  }
}

func startTesting(pc net.Conn, c net.Conn, received chan string, fileToSend string) {
  results, err := os.OpenFile("results.txt", os.O_WRONLY|os.O_CREATE, 0755)
  if err != nil {
    printer.Fatal(err)
  }
  defer results.Close()


  mtus := []int{100, 200, 400, 800, 1300, 1600, 3200, 6400, 12800, 25600, 51200}
  for _, partLen := range mtus {
    retrCount := 0
    sendMsg(c, READY)
    for {
      msg := <- received
      if msg == READY || msg == DO_RETRANSMIT {
        sendFile(fileToSend, partLen, pc)
        retrCount++
      } else if msg == FILE_RECEIVED {
        sendMsg(c, STATS)
        stats := <- received
        printer.Debug(stats, "Stats")
        ar := strings.Split(stats, ",")
        
        printer.Note(ar[0],"--- file sent")
        printer.Note(retrCount,"--- retransmission count")
        printer.Note(ar[1], "--- size (bytes)")
        printer.Note(ar[2], "--- time taken (s)")
        printer.Note(ar[3], "--- mean speed (kbps)")

        results.Write([]byte(fmt.Sprintf("mtu=%d size=%s time=%s speed=%s retries=%d\n", 
                  partLen, ar[1], ar[2], ar[3], retrCount)))
        results.Sync()

        break
      }
    }

  }

}

func usage() {
  println("Usage: client [FILE] [PART_LEN] [POST_URL]")
  os.Exit(1)
}

func main() {
  if len(os.Args) < 4 {
    usage()
  }

  fileToSend := os.Args[1]
  // partLen,_ := strconv.Atoi(os.Args[2])
  serverAddr := os.Args[3]

  printer.Debug("Client started")

  rand.Seed(time.Now().UnixNano())

  // Connect udp
  pc, err := net.Dial("udp", serverAddr+":"+serverPortUdp)
  if err != nil {
    printer.Fatal(err)
  }
  defer pc.Close()

  // Connect tcp
  c, err := net.Dial("tcp", serverAddr+":"+serverPortTcp)
  if err != nil {
    printer.Fatal(err)
  }
  defer c.Close()

  received := make(chan string)
  go readCommand(c, received)

  startTesting(pc, c, received, fileToSend)
}