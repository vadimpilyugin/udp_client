package main

import (
  "github.com/vadimpilyugin/debug_print_go"
  "net"
  "os"
  "io"
)

const (
  MTU = 1500
)

func usage() {
  println("Usage: client [FILE] [POST_URL]")
  os.Exit(1)
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

  buf := make([]byte, MTU)
  file, err := os.Open(fileToSend)
  if err != nil {
    printer.Fatal(err)
  }
  defer file.Close()

  //simple write
  for {
    _, err := file.Read(buf)
    if err == io.EOF {
      break
    }
  }
  // conn.Write([]byte("Hello from client"))
  _, err = conn.Write(buf)
  if err != nil {
    printer.Fatal(err, "conn.Write")
  }
}