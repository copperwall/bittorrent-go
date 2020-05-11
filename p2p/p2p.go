package p2p

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"runtime"
	"time"

	"github.com/copperwall/bittorrent-go/client"
	"github.com/copperwall/bittorrent-go/message"
	"github.com/copperwall/bittorrent-go/peers"
)

const MaxBlockSize = 16384
// MaxBacklog is the number of pending requests a client can have
const MaxBacklog = 10

// Torrent holds necessary information like PeerID, a list of Peers, the InfoHash,
// PieceHashes and name
type Torrent struct {
	Peers			[]peers.Peer
	PeerID			[20]byte
	InfoHash		[20]byte
	PieceHashes		[][20]byte
	PieceLength		int
	Length			int
	Name			string
}

type pieceWork struct {
	index	int
	hash 	[20]byte
	length 	int
}

type pieceResult struct {
	index int
	buf []byte
}

type pieceProgress struct {
	index 		int
	client 		*client.Client
	buf 		[]byte
	downloaded 	int
	requested 	int
	backlog 	int
}

func (state *pieceProgress) readMessage() error {
	msg, err := state.client.Read()

	if err != nil {
		return err
	}

	// keep-alive
	if msg == nil {
		return nil
	}

	switch msg.ID {
	case message.MsgUnchoke:
		state.client.Choked = false
	case message.MsgChoke:
		state.client.Choked = true
	case message.MsgHave:
		index, err := message.ParseHave(msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case message.MsgPiece:
		n, err := message.ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}

		state.downloaded += n
		state.backlog--
	}

	return nil
}

func (t *Torrent) Download(w io.WriterAt) error {
	fmt.Println("Starting download for", t.Name)

	// Start the work queue
	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult)

	// Place work pieces in the work queue
	for index, hash := range t.PieceHashes {
		length := t.calculatePieceSize(index)
		workQueue <- &pieceWork{index, hash, length}
	}

	// Kick off the workers
	for _, peer := range t.Peers {
		go t.startDownloadWorker(peer, workQueue, results)
	}

	// make a buffer the length of the entire torrent output
	// buf := make([]byte, t.Length)
	donePieces := 0

	for donePieces < len(t.PieceHashes) {
		res := <- results
		begin, _ := t.calculateBoundsForPiece(res.index)
		w.WriteAt(res.buf, int64(begin))
		// copy(buf[begin : end], res.buf)

		donePieces++

		percent := float64(donePieces) / float64(len(t.PieceHashes)) * 100
		numWorkers := runtime.NumGoroutine() - 1
		log.Printf("(%0.2f%%) Downloaded piece #%d from %d peers\n", percent, res.index, numWorkers)
	}

	close(workQueue)

	return nil
}

// Bounds are specified by the PieceLength and the total Length
// The second piece will begin at 2 * PieceLength.
// The end of the second piece will be at (2 * PieceLength) + PieceLength
// Unless it's at the end of the thing, then length is the rest of the total length
func (t *Torrent) calculateBoundsForPiece(index int) (int, int) {
	begin := index * t.PieceLength
	end := begin + t.PieceLength

	if end > t.Length {
		end = t.Length
	}

	return begin, end
}

// Length is determined by getting the different between the start and end of the piece.
func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)

	return end - begin
}

func (t *Torrent) startDownloadWorker(peer peers.Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	c, err := client.New(peer, t.PeerID, t.InfoHash)

	if err != nil {
		log.Printf("Could not handshake with %s. Disconnecting\n", peer.IP)
		return
	}

	defer c.Conn.Close()

	log.Printf("Completed handshake with %s\n", peer.IP)

	// Connections are immediately choked, so first we need to unchoke
	c.SendUnchoke()
	c.SendInterested()

	for pw := range workQueue {
		// If this peer doesn't have the piece, place it back on the queue for someone other 
		// peer to pick up.
		if !c.Bitfield.HasPiece(pw.index) {
			workQueue <- pw
			continue
		}

		buf, err := attemptDownloadPiece(c, pw)
		if err != nil {
			log.Println("Exiting", err)
			workQueue <- pw
			return
		}

		err = checkIntegrity(pw, buf)
		if err != nil {
			log.Printf("Piece #%d failed integrity check\n", pw.index)
		}

		c.SendHave(pw.index)
		results <- &pieceResult{pw.index, buf}
	}
}

func attemptDownloadPiece(client *client.Client, pw *pieceWork) ([]byte, error) {
	state := pieceProgress{
		index: 		pw.index,
		client: 	client,
		buf:		make([]byte, pw.length),
	}

	client.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	// Disable deadline after function finishes.
	defer client.Conn.SetDeadline(time.Time{})

	for state.downloaded < pw.length {
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize

				if pw.length - state.requested < blockSize {
					blockSize = pw.length - state.requested
				}

				err := client.SendRequest(pw.index, state.requested, blockSize)

				if err != nil {
					return nil, err
				}

				state.backlog++
				state.requested += blockSize
			}
		}

		err := state.readMessage()
		if err != nil {
			return nil, err
		}
	}

	return state.buf, nil
}

// The hash for the buf should match the pieceWork hash
func checkIntegrity(pw *pieceWork, buf []byte) error {
	sha := sha1.Sum(buf)

	if !bytes.Equal(sha[:], pw.hash[:]) {
		return fmt.Errorf("Index %d failed integrity check", pw.index)
	}

	return nil
}
