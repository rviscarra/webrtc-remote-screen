package rtc

import (
	"io"
)

type videoStreamer interface {
	start()
	close()
}

// RemoteScreenConnection Represents a WebRTC connection to a single peer
type RemoteScreenConnection interface {
	io.Closer
	ProcessOffer(offer string) (string, error)
}

// Service WebRTC service
type Service interface {
	CreateRemoteScreenConnection(screenIx int, fps int) (RemoteScreenConnection, error)
}
