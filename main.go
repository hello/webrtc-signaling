package main

import (
	"errors"
	"github.com/hello/webrtc-signaling/signaling"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"net"
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

const (
	port = ":6556"
)

type HelloSignaling struct {
	offer   string
	answers chan string
}

func (s *HelloSignaling) Join(ctx context.Context, answer *signaling.SdpAnswer) (*signaling.Empty, error) {
	log.Println("received join call")
	s.answers <- answer.GetSdp()
	log.Println("responding with empty")
	empty := &signaling.Empty{
		Block: true,
	}
	return empty, nil
}

func (s *HelloSignaling) Wait(empty *signaling.Empty, stream signaling.Signaling_WaitServer) error {
	log.Println("Waiting ....")
	offer := &signaling.SdpOffer{
		Sdp: s.offer,
	}
	log.Println("about to send offer on stream")
	err := stream.Send(offer)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Sent offer")
	return nil
}

func (s *HelloSignaling) Start(offer *signaling.SdpOffer, stream signaling.Signaling_StartServer) error {
	log.Println("Get SDP", offer.GetSdp())
	s.offer = offer.GetSdp()

	// Waiting for Answers
	log.Println("Waiting for answers")
wait:
	for {
		select {
		case a := <-s.answers:
			answer := &signaling.SdpAnswer{
				Sdp: a,
			}

			err := stream.Send(answer)
			log.Println("Sent answer")
			if err != nil {
				log.Fatal(err)
			}
			break wait

		}
	}
	log.Println("Start Done")
	return nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	hs := &HelloSignaling{
		offer:   "",
		answers: make(chan string, 0),
	}

	signaling.RegisterSignalingServer(s, hs)

	log.Println("Serving...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
