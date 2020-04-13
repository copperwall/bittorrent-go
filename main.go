package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

// TorrentFile : Everything we need lol
type TorrentFile struct {
  Announce string
  InfoHash [20]byte
  PieceHashes [][20]byte
  PieceLength int
  Length int
  Name string
}

type bencodeInfo struct {
  Pieces string `bencode:"pieces"`
  PieceLength int `bencode:"piece length"`
  Length int `bencode:"length"`
  Name string `bencode:"name"`
}

type bencodeTorrent struct {
  Announce string `bencode:"announce"`
  Info bencodeInfo `bencode:"info"`
}

func (info *bencodeInfo) hash() ([20]byte, error) {
  buf := bytes.Buffer{}

  err := bencode.Marshal(&buf, *info)

  if err != nil {
    return [20]byte{}, err
  }

  return sha1.Sum(buf.Bytes()), nil
}

func (info *bencodeInfo) splitPieces() ([][20]byte, error) {
  // split pieces into 20 byte sections
  hashLen := 20

  // Cast Pieces into a buf 
  piecesBuf := []byte(info.Pieces) 

  // If pieces isn't divisible by the sha1 hash length, we have a problem
  if len(piecesBuf) % hashLen != 0 {
    err := fmt.Errorf("Pieces of length %v isn't divisible by %v", len(piecesBuf), hashLen)

    return nil, err
  }

  numHashes := len(piecesBuf) / hashLen
  hashes := make([][20]byte, numHashes)

  for i := 0; i < numHashes; i++ {
    // The : in the second arg to copy is a [ : ]
    // For i = 0, would look like [0 : 20]
    // For i = 0, would look like [20 : 40]
    copy(hashes[i][:], piecesBuf[i * hashLen : (i + 1) * hashLen])
  }

  return hashes, nil
}

func (b *bencodeTorrent) toTorrentFile() (TorrentFile, error) {
  tf := TorrentFile{}

  infoHash, err := b.Info.hash()

  if err != nil {
    return TorrentFile{}, err
  }

  pieceHashes, err := b.splitPieces()

  if err != nil {
    return TorrentFile{}, err
  }

  tf.Announce = b.Announce
  tf.InfoHash = infoHash
  tf.PieceHashes = pieceHashes
  tf.PieceLength = b.Info.PieceLength
  tf.Length = b.Info.Length
  tf.Name = b.Info.Name

  return tf, nil
}

func main() {
  // Handle arguments
  args, err := validateArgs(os.Args[1:])

  if err != nil {
    fmt.Println(err)
    fmt.Println("Usage:", os.Args[0], "<filename>")
    os.Exit(1)
  }

  fmt.Println(args.filename)

  r, err := os.Open(args.filename)

  if err != nil {
    fmt.Println(err)
    os.Exit(1)
  }

  torrentInfo := bencodeTorrent{}
  bencode.Unmarshal(r, &torrentInfo)
  fmt.Println(torrentInfo.Announce)
  outfile := torrentInfo.Info.Name
  fmt.Println("Creating file at", outfile)
  _, createErr := os.Create(outfile)

  if createErr != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}

type arguments struct {
  filename string
}

const numArgs = 1

func validateArgs(args []string) (arguments, error) {
  if len(args) != numArgs {
    return arguments{}, fmt.Errorf("Error: Expected %v arguments, but got %v", numArgs, len(args))
  }

  return arguments{
    args[0],
  }, nil
}
