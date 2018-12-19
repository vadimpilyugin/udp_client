package main

import (
  "github.com/vadimpilyugin/debug_print_go"
  "net"
  "os"
  "fmt"
  "io/ioutil"
  "math/rand"
  "time"
  "strconv"
  "encoding/binary"
  "errors"
  "path"
)

const (
  MTU = 1460
  BUFLEN = 4096
  MAX_FN_LEN = 20
  LEN_ERR = "Filename is too long"
  FN_L = 1
  INDEX_LEN = 8
  HEADER_LEN = FN_L + MAX_FN_LEN + 2 * INDEX_LEN
)

type FilePart struct {
  Filename string
  PartNo int64
  NParts int64
  FilePart []byte
}

func usage() {
  println("Usage: client [FILE] [PART_LEN] [POST_URL]")
  os.Exit(1)
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

// func predictLen(fileToSend string, partLen int) int {
//   var encodedLen float64 = 4.0 / 3.0 * float64(partLen)
//   return int(encodedLen) + len(fileToSend) + 51
// }

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

  fileSize := int64(len(content))
  nParts := fileSize / int64(partLen)
  if fileSize % int64(partLen) > 0 {
    nParts += 1
  }
  partsSeq := randomPartsSeq(nParts)

  done := false
  for _, partNo := range partsSeq {
    buf, err := FilePart{
      Filename: path.Base(fileToSend),
      PartNo: partNo,
      NParts: nParts,
      FilePart: content[partNo * int64(partLen) : (partNo + 1) * int64(partLen)],
    }.MarshalBinary()

    if err != nil {
      printer.Fatal(err)
    }

    _, err = conn.Write(buf)
    if err != nil {
      printer.Error(err, "conn.Write")
    }
    printer.Debug(
      fmt.Sprintf("payload len=%d", len(buf)), 
      fmt.Sprintf("%d / %d", partNo, nParts),
    )
    if len(buf) > MTU {
      printer.Note("Outside of MTU limit!", fmt.Sprintf("Packet len=%d", len(buf)))
    }

    if done {
      break
    }

    partNo++
  }
}

func main() {
  if len(os.Args) < 4 {
    usage()
  }

  printer.Debug("Hello, world!")

  fileToSend := os.Args[1]
  partLen,_ := strconv.Atoi(os.Args[2])
  postUrl := os.Args[3]

  rand.Seed(time.Now().UnixNano())

  //Connect udp
  conn, err := net.Dial("udp", postUrl)
  if err != nil {
    printer.Fatal(err)
  }
  defer conn.Close()
  sendFile(fileToSend, partLen, conn)
}