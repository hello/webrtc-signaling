package main

import (
	"fmt"
	"github.com/hello/webrtc-signaling/signaling"
	"github.com/keroserene/go-webrtc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
)

type PhonePeer struct {
	pc     *webrtc.PeerConnection
	dc     *webrtc.DataChannel
	err    error
	sdp    string
	client signaling.SignalingClient
	msg    chan string
}

func (c *PhonePeer) sendOfferAndWait() {

	offer := &signaling.SdpOffer{
		Sdp: c.sdp,
	}

	ctx := context.Background()
	answerStream, err := c.client.Start(ctx, offer)

	if err != nil {
		log.Fatal(err)
	}

	for {
		fmt.Println("Waiting for answer")
		answer, err := answerStream.Recv()
		if err != nil {
			log.Fatal(err)
		}

		session := &webrtc.SessionDescription{
			Type: "answer", // TODO: this is janky
			Sdp:  answer.GetSdp(),
		}
		c.pc.SetRemoteDescription(session)
		break
	}
}

func (c *PhonePeer) init(pc *webrtc.PeerConnection) {

	// OnNegotiationNeeded is triggered when something important has occurred in
	// the state of PeerConnection (such as creating a new data channel), in which
	// case a new SDP offer must be prepared and sent to the remote peer.
	pc.OnNegotiationNeeded = func() {
		go c.CreateOffer()
	}
	// Once all ICE candidates are prepared, they need to be sent to the remote
	// peer which will attempt reaching the local peer through NATs.
	pc.OnIceComplete = func() {
		fmt.Println("Finished gathering ICE candidates.")
		if c.err != nil {
			log.Fatal(c.err)
		}
		go c.CreateOffer()
	}

	c.pc = pc

	dc, err := c.pc.CreateDataChannel("test", webrtc.Init{})
	if nil != err {
		fmt.Println("Unexpected failure creating Channel.")
		return
	}

	dc.OnOpen = func() {
		fmt.Println("Data Channel Opened!")
	}
	dc.OnClose = func() {
		fmt.Println("Data Channel closed.")
	}

	dc.OnMessage = func(msg []byte) {
		fmt.Println("received:", string(msg))
	}

	c.dc = dc

}
func (c *PhonePeer) CreateOffer() {
	offer, err := c.pc.CreateOffer() // blocking
	if err == nil {
		c.pc.SetLocalDescription(offer)
		session := c.pc.LocalDescription()
		c.sdp = session.Sdp
		c.sendOfferAndWait()
	}

	c.err = err
}

func main() {

	done := make(chan bool, 0)
	dialOptions := []grpc.DialOption{
		// grpc.WithTimeout(500 * time.Millisecond),
		grpc.WithInsecure(),
		grpc.WithUserAgent("grpc-go-client"),
	}
	ctx := context.Background()

	cc, err := grpc.DialContext(ctx, ":6556", dialOptions...)
	if err != nil {
		log.Fatal(err)
	}

	client := signaling.NewSignalingClient(cc)

	config := webrtc.NewConfiguration(
		webrtc.OptionIceServer("stun:stun.l.google.com:19302"))

	pc, err := webrtc.NewPeerConnection(config)
	if nil != err {
		fmt.Println("Failed to create PeerConnection.")
		return
	}

	peer := &PhonePeer{
		client: client,
	}

	peer.init(pc)

	<-done

}
