package main

import (
	"bytes"
	"fmt"
	"github.com/hello/webrtc-signaling/signaling"
	"github.com/keroserene/go-webrtc"
	"github.com/nfnt/resize"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"os"
	"reflect"
	"time"
)

var ASCIISTR = "MND8OZ$7I?+=~:,.."

type CameraPeer struct {
	pc     *webrtc.PeerConnection
	dc     *webrtc.DataChannel
	err    error
	sdp    string
	client signaling.SignalingClient
	img    image.Image
}

func (c *CameraPeer) waitForOfferAndSendAnswer() {
	ctx := context.Background()
	empty := &signaling.Empty{}
	log.Println("Waiting for offer...")
	waitClient, err := c.client.Wait(ctx, empty)
	if err != nil {
		log.Fatal(err)
	}

	for {

		log.Println("Waiting for offer")
		offer, err := waitClient.Recv()
		if err == io.EOF {
			log.Println("Got EOF")
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		session := &webrtc.SessionDescription{
			Type: "offer", // TODO: this is janky
			Sdp:  string(offer.GetSdp()),
		}
		log.Println("set remote description")
		setRemoteErr := c.pc.SetRemoteDescription(session)
		if setRemoteErr != nil {
			log.Fatal("setRemoteErr", setRemoteErr)
		}

		go c.CreateAnswer()

		break
	}

}
func (c *CameraPeer) init(pc *webrtc.PeerConnection) {

	// OnNegotiationNeeded is triggered when something important has occurred in
	// the state of PeerConnection (such as creating a new data channel), in which
	// case a new SDP offer must be prepared and sent to the remote peer.
	// Once all ICE candidates are prepared, they need to be sent to the remote
	// peer which will attempt reaching the local peer through NATs.

	pc.OnNegotiationNeeded = func() {
		log.Fatal("Negotiation needed")
	}

	pc.OnIceComplete = func() {
		log.Println("Finished gathering ICE candidates.")
		if c.err != nil {
			log.Fatal(c.err)
		}
	}

	c.pc = pc
	c.pc.OnDataChannel = func(channel *webrtc.DataChannel) {
		log.Println("Datachannel established by remote... ", channel.Label())
		c.dc = channel
		c.dc.OnOpen = func() {
			log.Println("Data Channel Opened!")
		}
		c.dc.OnClose = func() {
			log.Println("Data Channel closed.")
		}

		c.dc.OnMessage = func(msg []byte) {
			log.Println("received:", string(msg))
		}

	}
	c.waitForOfferAndSendAnswer()
}

func (c *CameraPeer) CreateAnswer() {
	ctx := context.Background()
	log.Println("creating answer")
	answer, err := c.pc.CreateAnswer() // blocking
	log.Println("Created answer")
	if err != nil {
		log.Fatal("failing", err)
	}
	if err == nil {
		c.pc.SetLocalDescription(answer)
		session := c.pc.LocalDescription()
		c.sdp = session.Sdp

		a := &signaling.SdpAnswer{
			Sdp: c.sdp,
		}
		empty, err := c.client.Join(ctx, a)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("block", empty.GetBlock())
	} else {
		log.Fatal(err)
	}
	c.err = err

	t := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-t.C:
			p := Convert2Ascii(ScaleImage(c.img, 40))
			c.dc.Send(p)
			log.Println("Image sent")
		}
	}
}

func ScaleImage(img image.Image, w int) (image.Image, int, int) {
	sz := img.Bounds()
	h := (sz.Max.Y * w * 10) / (sz.Max.X * 16)
	img = resize.Resize(uint(w), uint(h), img, resize.Lanczos3)
	return img, w, h
}

func Convert2Ascii(img image.Image, w, h int) []byte {
	table := []byte(ASCIISTR)
	buf := new(bytes.Buffer)

	for i := 0; i < h; i++ {
		for j := 0; j < w; j++ {
			g := color.GrayModel.Convert(img.At(j, i))
			y := reflect.ValueOf(g).FieldByName("Y").Uint()
			pos := int(y * 16 / 255)
			_ = buf.WriteByte(table[pos])
		}
		_ = buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func main() {

	f, err := os.Open("/Users/tim/Desktop/github.png")
	if err != nil {
		log.Fatal(err)
	}

	img, _, err := image.Decode(f)

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

	peer := &CameraPeer{
		client: client,
		img:    img,
	}

	peer.init(pc)

	<-done

}
