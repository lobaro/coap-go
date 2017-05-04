package coap

import (
	"bytes"
	"io"
	"sync"
)

type SafeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *SafeBuffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}
func (b *SafeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}
func (b *SafeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

func (b *SafeBuffer) Bytes() []byte {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Bytes()
}
func (b *SafeBuffer) Cap() int {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Cap()
}
func (b *SafeBuffer) Grow(n int) {
	b.m.Lock()
	defer b.m.Unlock()
	b.b.Grow(n)
}
func (b *SafeBuffer) Len() int {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Len()
}
func (b *SafeBuffer) Next(n int) []byte {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Next(n)
}
func (b *SafeBuffer) ReadByte() (c byte, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.ReadByte()
}
func (b *SafeBuffer) ReadBytes(delim byte) (line []byte, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.ReadBytes(delim)
}
func (b *SafeBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.ReadFrom(r)
}
func (b *SafeBuffer) ReadRune() (r rune, size int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.ReadRune()
}
func (b *SafeBuffer) ReadString(delim byte) (line string, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.ReadString(delim)
}
func (b *SafeBuffer) Reset() {
	b.m.Lock()
	defer b.m.Unlock()
	b.b.Reset()
}
func (b *SafeBuffer) Truncate(n int) {
	b.m.Lock()
	defer b.m.Unlock()
	b.b.Truncate(n)
}
func (b *SafeBuffer) UnreadByte() error {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.UnreadByte()
}
func (b *SafeBuffer) UnreadRune() error {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.UnreadRune()
}
func (b *SafeBuffer) WriteByte(c byte) error {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.WriteByte(c)
}
func (b *SafeBuffer) WriteRune(r rune) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.WriteRune(r)
}
func (b *SafeBuffer) WriteString(s string) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.WriteString(s)
}
func (b *SafeBuffer) WriteTo(w io.Writer) (n int64, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.WriteTo(w)
}
