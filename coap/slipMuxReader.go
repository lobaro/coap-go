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
	for {
		packet, frame, err := r.r.ReadPacket()

		if frame == slip.FRAME_DIAGNOSTIC {
			if SlipMuxLogDiagnostic {
				log.WithField("message", strings.TrimSpace(string(packet))).Debug("SlipMux Diagnostic")
			}
			continue
		}

		if frame == slip.FRAME_COAP {
			return packet, false, err
		}

		if err != nil {
			return packet, false, err
		}

		// silently ignore unhandled packets.
		log.WithError(err).WithField("packet", packet).WithField("frameType", frame).Info("Unknown SlipMux packet")

		continue
	}
}

func (w *SlipMuxWriter) WritePacket(p []byte) (err error) {
	return w.w.WritePacket(slip.FRAME_COAP, p)
}
