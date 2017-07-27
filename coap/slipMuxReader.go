package coap

import (
	"io"

	"strings"

	"github.com/Lobaro/slip"
)

var SlipMuxLogDiagnostic bool

type SlipMuxReader struct {
	r *slip.SlipMuxReader
}

func NewSlipMuxReader(reader io.Reader) *SlipMuxReader {
	return &SlipMuxReader{r: slip.NewSlipMuxReader(reader)}
}

type SlipMuxWriter struct {
	w *slip.SlipMuxWriter
}

func NewSlipMuxWriter(writer io.Writer) *SlipMuxWriter {
	return &SlipMuxWriter{w: slip.NewSlipMuxWriter(writer)}
}

func (r *SlipMuxReader) ReadPacket() ([]byte, bool, error) {
	packet, frame, err := r.r.ReadPacket()

	if frame == slip.FRAME_DIAGNOSTIC {
		if SlipMuxLogDiagnostic {
			log.WithField("message", strings.TrimSpace(string(packet))).Debug("SlipMux Diagnostic")
		}
		return nil, false, nil
	}

	if frame == slip.FRAME_COAP {
		return packet, false, err
	}
	// silently ignore unhandled packets.
	log.WithField("packet", packet).WithField("frameType", frame).Debug("Unknown SlipMux packet")

	return nil, false, err
}

func (w *SlipMuxWriter) WritePacket(p []byte) (err error) {
	return w.w.WritePacket(slip.FRAME_COAP, p)
}
