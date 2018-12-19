package main

import (
  "github.com/vadimpilyugin/debug_print_go"
  "net"
  "os"
  "encoding/json"
  "fmt"
  "io"
)

const (
  MTU = 1460
)

type FilePart struct {
  Filename string
  PartNo int64
  NParts int64
  FilePart []byte
}

func usage() {
  println("Usage: client [FILE] [POST_URL]")
  os.Exit(1)
}

func predictLen(fileToSend string, partLen int) int {
  var encodedLen float64 = 4.0 / 3.0 * float64(partLen)
  return int(encodedLen) + len(fileToSend) + 51
}

func sendFile(fileToSend string, partLen int, conn net.Conn) {
  file, err := os.Open(fileToSend)
  if err != nil {
    printer.Fatal(err)
  }
  defer file.Close()

  fp := make([]byte, partLen)
  done := false
  var partNo int64 = 0
  stats, err := file.Stat()
  if err != nil {
    printer.Fatal(err)
  }
  nParts := stats.Size() / int64(partLen)
  if stats.Size() % int64(partLen) > 0 {
    nParts += 1
  }
  for {
    n, err := file.Read(fp)
    if err == io.EOF {
      done = true
      if n == 0 {
        break
      }
    } else if err != nil {
      printer.Fatal(err)
    }

    buf, err := json.Marshal(FilePart{
      Filename: fileToSend,
      PartNo: partNo,
      NParts: nParts,
      FilePart: fp[:n],
    })
    if err != nil {
      printer.Fatal(err)
    }

    _, err = conn.Write(buf)
    if err != nil {
      printer.Error(err, "conn.Write")
    }
    printer.Debug(
      fmt.Sprintf("predicted payload len=%d, payload len=%d", predictLen(fileToSend, partLen), len(buf)), 
      fmt.Sprintf("%d / %d", partNo, nParts),
    )

    if done {
      break
    }

    partNo++
  }
}

func main() {
  if len(os.Args) < 3 {
    usage()
  }

  printer.Debug("Hello, world!")

  fileToSend := os.Args[1]
  postUrl := os.Args[2]

  //Connect udp
  conn, err := net.Dial("udp", postUrl)
  if err != nil {
    printer.Fatal(err)
  }
  defer conn.Close()

  partLen := 1050
  pred := predictLen(fileToSend, partLen)
  if pred > MTU {
    printer.Note("May be outside of MTU limit!", fmt.Sprintf("Packet len=%d", pred))
  }
  sendFile(fileToSend, partLen, conn)
}