package ogg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var PageHeaderPattern = [4]byte{'O', 'g', 'g', 'S'}

var ErrCorruptStream = errors.New("ogg: corrupt stream")
var ErrChecksum = errors.New("ogg: wrong checksum")

const (
	HeaderFlagContinuedPacket   = 1
	HeaderFlagBeginningOfStream = 2
	HeaderFlagEndOfStream       = 4
)

type pageHeader struct {
	CapturePattern          [4]byte
	StreamStructureVersion  uint8
	HeaderTypeFlag          byte
	AbsoluteGranulePosition uint64
	StreamSerialNumber      uint32
	PageSequenceNumber      uint32
	PageChecksum            uint32
	PageSegments            uint8
}

type PageHeader struct {
	pageHeader
	SegmentTable   []uint8
	headerChecksum uint32
}

type Packet struct {
	StreamSerialNumber uint32
	Content            []byte
}

func (h *PageHeader) ReadFrom(r io.Reader) error {
	data := make([]byte, 27)
	_, err := io.ReadFull(r, data)
	if err != nil {
		return err
	}
	binary.Read(bytes.NewReader(data), binary.LittleEndian, &h.pageHeader)
	if h.CapturePattern != PageHeaderPattern {
		return ErrCorruptStream
	}
	h.SegmentTable = make([]byte, h.PageSegments)
	_, err = io.ReadFull(r, h.SegmentTable)
	if err != nil {
		return err
	}
	data[22], data[23], data[24], data[25] = 0, 0, 0, 0
	h.headerChecksum = crcUpdate(0, data)
	h.headerChecksum = crcUpdate(h.headerChecksum, h.SegmentTable)
	return nil
}

func (h *PageHeader) IsFirstPage() bool { return h.HeaderTypeFlag&HeaderFlagBeginningOfStream != 0 }
func (h *PageHeader) IsLastPage() bool  { return h.HeaderTypeFlag&HeaderFlagEndOfStream != 0 }
func (h *PageHeader) IsContinue() bool  { return h.HeaderTypeFlag&HeaderFlagContinuedPacket != 0 }
