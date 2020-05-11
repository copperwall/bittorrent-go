package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/copperwall/bittorrent-go/p2p"
	"github.com/copperwall/bittorrent-go/peers"
	"github.com/jackpal/bencode-go"
)

const Port = 6881

// TorrentFile : Everything we need lol
type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

type bencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type bencodeTorrent struct {
	Announce string      `bencode:"announce"`
	Info     bencodeInfo `bencode:"info"`
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
	if len(piecesBuf)%hashLen != 0 {
		err := fmt.Errorf("Pieces of length %v isn't divisible by %v", len(piecesBuf), hashLen)

		return nil, err
	}

	numHashes := len(piecesBuf) / hashLen
	hashes := make([][20]byte, numHashes)

	for i := 0; i < numHashes; i++ {
		// The : in the second arg to copy is a [ : ]
		// For i = 0, would look like [0 : 20]
		// For i = 0, would look like [20 : 40]
		copy(hashes[i][:], piecesBuf[i*hashLen:(i+1)*hashLen])
	}

	return hashes, nil
}

func (b *bencodeTorrent) toTorrentFile() (TorrentFile, error) {
	tf := TorrentFile{}

	infoHash, err := b.Info.hash()

	if err != nil {
		return TorrentFile{}, err
	}

	pieceHashes, err := b.Info.splitPieces()

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

func (t *TorrentFile) toTrackerURL(peerID [20]byte, port uint16) (string, error) {
	announceURL, err := url.Parse(t.Announce)

	if err != nil {
		return "", err
	}

	// Build query params
	params := url.Values{
		// NOTE: What if I just don't include the slice portion?
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}

	// Append query params to announce base url and return
	// the stringified version.
	announceURL.RawQuery = params.Encode()
	return announceURL.String(), nil
}

type bencodeTrackerResp struct {
	Interval 	int 	`bencode:"interval"`
	Peers 		string	`bencode:"peers"`
}

func (t *TorrentFile) requestPeers(peerID [20]byte, port uint16) ([]peers.Peer, error) {
	url, err := t.toTrackerURL(peerID, port)

	if err != nil {
		return nil, err
	}
	c := &http.Client{Timeout: 15 * time.Second}

	log.Println("Asking for peers from tracker at url", url)
	resp, err := c.Get(url)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	fmt.Println("Lenght", resp)
	trackerResp := bencodeTrackerResp{}
	err = bencode.Unmarshal(resp.Body, &trackerResp)

	fmt.Println(trackerResp)
	if err != nil {
		return nil, err
	}

	return peers.Unmarshal([]byte(trackerResp.Peers))
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

	tf, err := torrentInfo.toTorrentFile()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(tf.toTrackerURL(tf.PieceHashes[0], 6881))

	var peerID [20]byte
	_, randErr := rand.Read(peerID[:])

	if randErr != nil {
		fmt.Println(randErr)
		os.Exit(1)
	}

	peers, err := tf.requestPeers(peerID, Port)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(peers) == 0 {
		fmt.Println("Found no peers, cannot download.")
		os.Exit(0)
	}

	fmt.Println(peers)

	torrent := p2p.Torrent{
		Peers: peers,
		PeerID: peerID,
		InfoHash: tf.InfoHash,
		PieceHashes: tf.PieceHashes,
		PieceLength: tf.PieceLength,
		Length: tf.Length,
		Name: tf.Name,
	}

	tempFileName := torrent.Name + ".download"

	tempFile, err := os.Create(tempFileName)
	err = torrent.Download(tempFile)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Rename(tempFileName, torrent.Name)

	// outFile, err := os.Create(torrent.Name)
	// buf, err := torrent.Download()
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }

	// defer outFile.Close()

	// _, err = outFile.Write(buf)

	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }

	fmt.Println("Holy shit did that work?")
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
